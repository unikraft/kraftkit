// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package list

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	gcrname "github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
)

type ListOptions struct {
	All    bool   `long:"all" usage:"Also show available official images"`
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`

	metro string
	token string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ListOptions{}, cobra.Command{
		Short:   "List all images at a metro for your account",
		Use:     "list",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Long: heredoc.Doc(`
			List all images at a metro for your account.
		`),
		Example: heredoc.Doc(`
			# List images in your account.
			$ kraft cloud image list

			# List images in your account along with available official images.
			$ kraft cloud image list --all

			# List all images in your account in JSON format.
			$ kraft cloud image list -o json
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-img",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *ListOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	return nil
}

func (opts *ListOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewImagesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	resp, err := client.WithMetro(opts.metro).List(ctx)
	if err != nil {
		return fmt.Errorf("could not list images: %w", err)
	}
	images, err := resp.AllOrErr()
	if err != nil {
		return fmt.Errorf("could not list images: %w", err)
	}

	filteredImages := images[:0]

imgloop:
	for _, image := range images {
		if len(image.Tags) == 0 {
			continue
		}

		var name string
		for _, taggedImgRef := range image.Tags {
			tag, err := parseTagReference(taggedImgRef)
			if err != nil {
				log.G(ctx).Warn("Invalid tagged image reference: ", err)
				continue
			}
			if name = tag.RepositoryStr(); isOfficial(name) && !opts.All {
				continue imgloop
			}
		}

		filteredImages = append(filteredImages, image)
	}

	if opts.Output == "json" {
		return printJSON(ctx, filteredImages)
	}

	// Sort by digest lexically. This ensures that comparisons between versions are symmetric.
	sort.Slice(filteredImages, func(i, j int) bool {
		return filteredImages[i].Digest < filteredImages[j].Digest
	})

	err = iostreams.G(ctx).StartPager()
	if err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	}

	defer iostreams.G(ctx).StopPager()

	cs := iostreams.G(ctx).ColorScheme()
	table, err := tableprinter.NewTablePrinter(ctx,
		tableprinter.WithMaxWidth(iostreams.G(ctx).TerminalWidth()),
		tableprinter.WithOutputFormatFromString(opts.Output),
	)
	if err != nil {
		return err
	}

	// Header row
	table.AddField("NAME", cs.Bold)
	table.AddField("VERSION", cs.Bold)
	if opts.Output != "table" {
		table.AddField("APP ARGS", cs.Bold)
		table.AddField("KERNEL ARGS", cs.Bold)
	}
	table.AddField("SIZE", cs.Bold)
	table.EndRow()

	for _, image := range filteredImages {
		var name string
		versions := make([]string, 0, len(image.Tags))

		for _, taggedImgRef := range image.Tags {
			tag, err := parseTagReference(taggedImgRef)
			if err != nil {
				log.G(ctx).Warn("Invalid tagged image reference: ", err)
				continue
			}

			name = tag.RepositoryStr()
			versions = append(versions, tag.TagStr())
		}

		slices.Sort(versions)

		table.AddField(name, nil)
		table.AddField(strings.Join(versions, ", "), nil)

		if opts.Output != "table" {
			table.AddField(image.Args, nil)
			table.AddField(image.KernelArgs, nil)
		}

		table.AddField(humanize.Bytes(uint64(image.SizeInBytes)), nil)

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// parseTagReference parses the given tagged image reference into a name.Tage.
// The input is expected to be in the format "[namespace/]image:tag" without
// the registry host.
func parseTagReference(tref string) (gcrname.Tag, error) {
	// Shortest possible string that can be parseable as a registry domain.
	// https://github.com/google/go-containerregistry/blob/v0.19.0/pkg/name/repository.go#L81-L91
	const sentinelRegHost = "::"

	t, err := gcrname.NewTag(tref, gcrname.WithDefaultRegistry(sentinelRegHost))
	if err != nil {
		return gcrname.Tag{}, fmt.Errorf("parsing image reference: %w", err)
	}

	// Special case: the ref string is namespaced with a valid hostname,
	// resulting in that namespace being parsed as the image's registry.
	//
	// Example:
	//
	//	(no registry/)user.unikraft.io/myapp      -> {reg:user.unikraft.io, img:myapp}
	//	              │             └── image
	//	              └── image namespace
	//
	if t.RegistryStr() != sentinelRegHost {
		return parseTagReference(sentinelRegHost + "/" + tref)
	}

	return t, nil
}

// isOfficial naively differentiates between official and non-official images.
func isOfficial(repo string) bool {
	return !isNamespacedRepository(repo)
}

// isNamespacedRepository returns whether the given image repository is
// namespaced.
func isNamespacedRepository(repo string) bool {
	const regNsDelimiter = '/'
	return strings.ContainsRune(repo, regNsDelimiter)
}

func printJSON(ctx context.Context, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("serializing data to JSON: %w", err)
	}
	fmt.Fprintln(iostreams.G(ctx).Out, string(b))
	return nil
}

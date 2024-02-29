// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package list

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
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
	All    bool   `long:"all" usage:"Show all images by their digest"`
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

			# List all images in your account.
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

	images, err := client.WithMetro(opts.metro).List(ctx)
	if err != nil {
		return fmt.Errorf("could not list images: %w", err)
	}

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

	// Sort the features lexically.  This ensures that comparisons between
	// versions are symmetric.
	sort.Slice(images, func(i, j int) bool {
		return images[i].Digest < images[j].Digest
	})

	// Header row
	table.AddField("NAME", cs.Bold)
	table.AddField("VERSION", cs.Bold)
	if opts.Output != "table" {
		table.AddField("PUBLIC", cs.Bold)
		table.AddField("ARGS", cs.Bold)
	}
	table.AddField("SIZE", cs.Bold)
	table.EndRow()

	for _, image := range images {
		if len(image.Tags) == 0 && !opts.All {
			continue
		}

		var name string
		var versions []string

		if opts.All {
			split := strings.Split(image.Digest, "@sha256:")
			name = split[0]
			versions = append(versions, split[1])
		}

		if len(image.Tags) > 0 {
			for _, tag := range image.Tags {
				split := strings.Split(tag, ":")
				name = split[0]
				versions = append(versions, split[1])
			}
		}

		slices.Sort[[]string](versions)

		table.AddField(name, nil)
		table.AddField(strings.Join(versions, ", "), nil)

		if opts.Output != "table" {
			table.AddField(fmt.Sprintf("%v", image.Public), nil)
			table.AddField(image.Args, nil)
		}

		table.AddField(humanize.Bytes(uint64(image.SizeInBytes)), nil)

		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

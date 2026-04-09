// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package list

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	gcrname "github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"

	"unikraft.com/cloud/sdk/controlplane"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
)

type ListOptions struct {
	All    bool   `long:"all" usage:"Also show available official images"`
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list,raw" default:"table"`

	token         string
	allowInsecure bool
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ListOptions{}, cobra.Command{
		Short:   "List all images for your account",
		Use:     "list",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			# List images in your account.
			$ kraft cloud image list

			# List images in your account along with available official images.
			$ kraft cloud image list --all

			# List all images in your account in JSON format.
			$ kraft cloud image list -o json
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-image",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *ListOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, nil, &opts.token, &opts.allowInsecure)
	if err != nil {
		return fmt.Errorf("could not populate token: %w", err)
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	return nil
}

func (opts *ListOptions) Run(ctx context.Context, args []string) error {
	return opts.runControlPlane(ctx)
}

func (opts *ListOptions) runControlPlane(ctx context.Context) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, strings.TrimSpace(opts.token))
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	clientOpts := []controlplane.ClientOption{
		controlplane.WithAllowInsecure(opts.allowInsecure),
		controlplane.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	}

	client := controlplane.NewClient(clientOpts...)

	details := true
	resp, err := client.ListImages(ctx, controlplane.ListImagesOpts{Details: &details})
	if err != nil {
		return fmt.Errorf("could not list images: %w", err)
	}

	if opts.Output == "raw" {
		fmt.Fprintln(iostreams.G(ctx).Out, string(resp.RawBody()))
		return nil
	}

	images := collectImageItems(resp)
	if opts.All {
		respOfficial, err := client.ListImages(ctx, controlplane.ListImagesOpts{Details: &details, Namespace: []string{"official"}})
		if err != nil {
			return fmt.Errorf("could not list official images: %w", err)
		}
		images = append(images, collectImageItems(respOfficial)...)
	}

	return opts.renderImages(ctx, images)
}

type imageRow struct {
	Digest      string
	Tags        []string
	SizeInBytes int64
}

func collectImageItems(resp *controlplane.Response[controlplane.ListImagesResponseData]) []imageRow {
	if resp == nil || resp.Data == nil {
		return nil
	}

	imagesByKey := make(map[string]*imageRow, len(resp.Data.Images))
	for _, image := range resp.Data.Images {
		name := ""
		if image.Name != nil {
			name = strings.TrimSpace(*image.Name)
		}
		if name == "" {
			continue
		}

		for _, tag := range image.Tags {
			tagName := ""
			if tag.Name != nil {
				tagName = strings.TrimSpace(*tag.Name)
			}
			if tagName == "" {
				continue
			}

			ref := fmt.Sprintf("%s:%s", name, tagName)
			digest := strings.TrimSpace(derefString(tag.Digest))
			key := name + "|" + digest
			if digest == "" {
				key = name + "|" + tagName + "|nodigest"
			}

			item, ok := imagesByKey[key]
			if !ok {
				item = &imageRow{Digest: digest}
				imagesByKey[key] = item
			}

			sizeInBytes := int64(0)
			if tag.Size != nil {
				sizeInBytes = int64(*tag.Size)
			}
			if sizeInBytes > item.SizeInBytes {
				item.SizeInBytes = sizeInBytes
			}
			item.Tags = append(item.Tags, ref)
		}
	}

	images := make([]imageRow, 0, len(imagesByKey))
	for _, item := range imagesByKey {
		if len(item.Tags) == 0 {
			continue
		}
		images = append(images, *item)
	}

	return images
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (opts *ListOptions) renderImages(ctx context.Context, images []imageRow) error {
	err := iostreams.G(ctx).StartPager()
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

	// Sort by namespace (official last), then repo, then tag.
	slices.SortFunc(images, func(left, right imageRow) int {
		leftRepo, leftTag := repoAndTag(left)
		rightRepo, rightTag := repoAndTag(right)

		leftOfficial := opts.All && isOfficialNamespace(leftRepo)
		rightOfficial := opts.All && isOfficialNamespace(rightRepo)
		if leftOfficial != rightOfficial {
			if leftOfficial {
				return 1
			}
			return -1
		}

		if leftRepo != rightRepo {
			return strings.Compare(leftRepo, rightRepo)
		}
		return strings.Compare(leftTag, rightTag)
	})

	// Header row
	table.AddField("NAME", cs.Bold)
	table.AddField("VERSION", cs.Bold)
	if opts.Output != "table" {
		table.AddField("DIGEST", cs.Bold)
	}
	table.AddField("SIZE", cs.Bold)
	table.EndRow()

imgloop:
	for _, image := range images {
		if len(image.Tags) == 0 {
			continue
		}

		var name string
		versions := make([]string, 0, len(image.Tags))

		for _, taggedImgRef := range image.Tags {
			tag, err := parseTagReference(taggedImgRef)
			if err != nil {
				log.G(ctx).Warn("Invalid tagged image reference: ", err)
				continue
			}

			repo := tag.RepositoryStr()
			if isOfficial(repo) && !opts.All {
				continue imgloop
			}
			if name == "" {
				name = repo
				if isOfficialNamespace(name) {
					name = strings.TrimPrefix(name, "official/")
					if name == "official" {
						name = ""
					}
				}
			}

			versions = append(versions, tag.TagStr())
		}

		if len(versions) == 0 || name == "" {
			continue
		}

		slices.Sort(versions)

		table.AddField(name, nil)
		table.AddField(strings.Join(versions, ", "), nil)

		if opts.Output != "table" {
			table.AddField(image.Digest, nil)
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

func isOfficialNamespace(repo string) bool {
	return repo == "official" || strings.HasPrefix(repo, "official/")
}

func repoAndTag(image imageRow) (string, string) {
	var (
		bestRepo string
		bestTag  string
		found    bool
	)

	for _, ref := range image.Tags {
		parsed, err := parseTagReference(ref)
		if err != nil {
			continue
		}

		repo := parsed.RepositoryStr()
		tag := parsed.TagStr()

		if !found || repo < bestRepo || (repo == bestRepo && tag < bestTag) {
			bestRepo = repo
			bestTag = tag
			found = true
		}
	}

	if !found {
		return "", ""
	}

	return bestRepo, bestTag
}

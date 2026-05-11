// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package list

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

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

	metro         string
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
	metro, token, allowInsecure, err := utils.GetMetroToken(cmd)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	opts.metro = metro
	opts.token = token
	opts.allowInsecure = allowInsecure

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

	if opts.Output == "raw" || opts.Output == "json" {
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

	// Also query the platform /v1/image-store endpoint to find images that may
	// not yet be visible via the controlplane.
	platformImages, err := opts.listPlatformImages(ctx, auth)
	if err != nil {
		log.G(ctx).Debugf("could not list platform images: %v", err)
	} else {
		images = mergeImageRows(images, platformImages)
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

// listPlatformImages queries the platform GET /v1/image-store endpoint for the
// configured metro and returns the results.
func (opts *ListOptions) listPlatformImages(ctx context.Context, auth *config.AuthConfig) ([]imageRow, error) {
	if opts.metro == "" {
		return nil, fmt.Errorf("no metro configured")
	}

	endpoint := opts.metro
	if !strings.Contains(endpoint, "://") {
		// If the metro value looks like a hostname (contains a dot), use it
		// directly as the API host. Otherwise treat it as a short metro code.
		if strings.Contains(endpoint, ".") {
			endpoint = "https://" + endpoint
		} else {
			endpoint = fmt.Sprintf("https://api.%s.unikraft.cloud", endpoint)
		}
	}
	endpoint = strings.TrimRight(endpoint, "/")

	// Strip any trailing /v1 path since we add it ourselves.
	endpoint = strings.TrimSuffix(endpoint, "/v1")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/v1/image-store", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.GetKraftCloudTokenAuthConfig(*auth))

	httpClient := &http.Client{Timeout: 30 * time.Second}
	if opts.allowInsecure {
		httpClient.Transport = &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d (%s) from %s", resp.StatusCode, resp.Status, req.URL.String())
	}

	var result struct {
		Data struct {
			Images []platformImage `json:"images"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	var images []imageRow
	for _, image := range result.Data.Images {
		row := platformImageToRow(image)
		if row != nil {
			images = append(images, *row)
		}
	}

	return images, nil
}

type platformImage struct {
	URL         *string  `json:"url"`
	Tags        []string `json:"tags"`
	SizeInBytes *int64   `json:"size_in_bytes"`
}

func platformImageToRow(image platformImage) *imageRow {
	row := &imageRow{}

	if image.SizeInBytes != nil {
		row.SizeInBytes = *image.SizeInBytes
	}

	var host, repo string
	if image.URL != nil {
		urlStr := *image.URL
		// URL format: "host/namespace/image@sha256:..."
		// Extract digest.
		if idx := strings.LastIndex(urlStr, "@"); idx != -1 {
			row.Digest = urlStr[idx+1:]
			urlStr = urlStr[:idx]
		}
		// Extract host and repo.
		if idx := strings.Index(urlStr, "/"); idx != -1 {
			host = urlStr[:idx]
			repo = urlStr[idx+1:]
		}
	}

	// Derive the index host from the image host by prepending "index.".
	indexHost := host
	if host != "" && !strings.HasPrefix(host, "index.") {
		indexHost = "index." + host
	}

	for _, tag := range image.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || strings.HasPrefix(tag, "sha256:") {
			continue
		}
		if repo != "" {
			row.Tags = append(row.Tags, indexHost+"/"+repo+":"+tag)
		}
	}

	// Skip images without tags (dangling images).
	if len(row.Tags) == 0 {
		return nil
	}

	return row
}

// mergeImageRows merges platform images into the existing image list,
// deduplicating by tag reference. Controlplane results take precedence.
// If a platform image has some tags already in controlplane and some that
// are new, only the new tags are kept.
func mergeImageRows(existing, platform []imageRow) []imageRow {
	seen := make(map[string]bool, len(existing))
	for _, img := range existing {
		for _, tag := range img.Tags {
			seen[tag] = true
		}
	}

	for _, img := range platform {
		// Filter out tags already known from controlplane.
		var newTags []string
		for _, tag := range img.Tags {
			if !seen[tag] {
				newTags = append(newTags, tag)
			}
		}
		if len(newTags) == 0 {
			continue
		}
		for _, tag := range newTags {
			seen[tag] = true
		}
		img.Tags = newTags
		existing = append(existing, img)
	}

	return existing
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

	// Sort: controlplane first (by repo, then tag), then platform (by repo,
	// then tag), then official last.
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

		// Platform images (with host prefix) sort after controlplane images.
		leftPlatform := isPlatformRepo(leftRepo)
		rightPlatform := isPlatformRepo(rightRepo)
		if leftPlatform != rightPlatform {
			if leftPlatform {
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

		// Images without usable tags are skipped because no display name can be derived.
		if name == "" {
			continue
		}

		slices.Sort(versions)
		if len(versions) == 0 {
			continue
		}

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

// isPlatformRepo returns true if the repo has a platform index host prefix
// in its first path component (e.g. "index.fra.unikraft.cloud/user/image").
func isPlatformRepo(repo string) bool {
	host := repo
	if i := strings.IndexRune(repo, '/'); i >= 0 {
		host = repo[:i]
	}

	return strings.HasPrefix(host, "index.") || strings.HasSuffix(host, ".unikraft.cloud")
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

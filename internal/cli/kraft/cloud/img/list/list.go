// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package list

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcimage "sdk.kraft.cloud/image"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
)

type ListOptions struct {
	Output string `long:"output" short:"o" usage:"Set output format" default:"table"`

	metro string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ListOptions{}, cobra.Command{
		Short:   "List all images at a metro for your account",
		Use:     "ls",
		Aliases: []string{"list"},
		Long: heredoc.Doc(`
			List all images in your account.
		`),
		Example: heredoc.Doc(`
			# List all images in your account.
			$ kraft cloud img ls
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
	opts.metro = cmd.Flag("metro").Value.String()
	if opts.metro == "" {
		opts.metro = os.Getenv("KRAFTCLOUD_METRO")
	}
	if opts.metro == "" {
		return fmt.Errorf("kraftcloud metro is unset")
	}
	log.G(cmd.Context()).WithField("metro", opts.metro).Debug("using")
	return nil
}

func (opts *ListOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudLoginFromContext(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kcimage.NewImagesClient(
		kraftcloud.WithToken(auth.Token),
	)

	images, err := client.WithMetro(opts.metro).List(ctx, map[string]interface{}{})
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

	// Header row
	table.AddField("IMAGE", cs.Bold)
	table.AddField("PUBLIC", cs.Bold)
	table.AddField("ROOTFS", cs.Bold)
	table.AddField("ARGS", cs.Bold)
	table.AddField("SIZE", cs.Bold)
	table.EndRow()

	for _, image := range images {
		if len(image.Tags) > 0 {
			table.AddField(image.Tags[0], nil)
		} else {
			table.AddField(image.Digest, nil)
		}
		table.AddField(fmt.Sprintf("%v", image.Public), nil)
		table.AddField(fmt.Sprintf("%v", image.Initrd), nil)
		table.AddField(strings.TrimSpace(fmt.Sprintf("%s -- %s", image.KernelArgs, image.Args)), nil)
		table.AddField(humanize.Bytes(uint64(image.SizeInBytes)), nil)
		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

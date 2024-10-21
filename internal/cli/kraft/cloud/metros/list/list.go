// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package list

import (
	"context"
	"fmt"
	"sort"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	cloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
)

type ListOptions struct {
	Status bool   `long:"status" short:"s" usage:"Also display the status of the metros"`
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ListOptions{}, cobra.Command{
		Short:   "List metros on UnikraftCloud",
		Use:     "list",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Long: heredoc.Doc(`
			List metros on cloud.
		`),
		Example: heredoc.Doc(`
		# List metros available.
		$ kraft cloud metro list

		# List metros available in list format.
		$ kraft cloud metro list -o list

		# List metros available and their status.
		$ kraft cloud metro list --status
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-metro",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *ListOptions) Pre(cmd *cobra.Command, _ []string) error {
	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	return nil
}

func (opts *ListOptions) Run(ctx context.Context, args []string) error {
	client := cloud.NewMetrosClient()

	metros, err := client.List(ctx, opts.Status)
	if err != nil {
		return fmt.Errorf("could not list metros: %w", err)
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

	// Sort the metros by delay.
	if opts.Status {
		sort.Slice(metros, func(i, j int) bool {
			return metros[i].Delay < metros[j].Delay
		})

		// Move the offline metros to the end of the list.
		sort.SliceStable(metros, func(i, j int) bool {
			return metros[i].Online && !metros[j].Online
		})
	}

	// Header row
	table.AddField("CODE", cs.Bold)
	table.AddField("IPV4", cs.Bold)
	table.AddField("LOCATION", cs.Bold)
	table.AddField("PROXY", cs.Bold)
	if opts.Status {
		table.AddField("STATUS", cs.Bold)
		table.AddField("PING", cs.Bold)
	}
	table.EndRow()

	for _, metro := range metros {
		table.AddField(metro.Code, nil)
		table.AddField(metro.Ipv4, nil)
		table.AddField(metro.Location, nil)
		table.AddField(metro.Proxy, nil)
		if opts.Status {
			if metro.Online {
				table.AddField("online", cs.Green)
			} else {
				table.AddField("offline", cs.Red)
			}
			table.AddField(fmt.Sprintf("%d MS", metro.Delay.Milliseconds()), nil)
		}
		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

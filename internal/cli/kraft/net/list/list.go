// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package list

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"

	networkapi "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/network"
)

type ListOptions struct {
	Long   bool   `long:"long" short:"l" usage:"Show more information"`
	Output string `long:"output" short:"o" usage:"Set output format" default:"table"`
	driver string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ListOptions{}, cobra.Command{
		Short:   "List machine networks",
		Use:     "ls [FLAGS]",
		Aliases: []string{"list"},
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "net",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *ListOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.driver = cmd.Flag("driver").Value.String()
	return nil
}

func (opts *ListOptions) Run(cmd *cobra.Command, _ []string) error {
	var err error

	ctx := cmd.Context()

	strategy, ok := network.Strategies()[opts.driver]
	if !ok {
		return fmt.Errorf("unsupported network driver strategy: %s", opts.driver)
	}

	controller, err := strategy.NewNetworkV1alpha1(ctx)
	if err != nil {
		return err
	}

	networks, err := controller.List(ctx, &networkapi.NetworkList{})
	if err != nil {
		return err
	}

	type netTable struct {
		id      string
		name    string
		network string
		driver  string
		status  networkapi.NetworkState
	}

	var items []netTable

	for _, network := range networks.Items {
		addr := &net.IPNet{
			IP:   net.ParseIP(network.Spec.Gateway),
			Mask: net.IPMask(net.ParseIP(network.Spec.Netmask)),
		}
		items = append(items, netTable{
			id:      string(network.UID),
			name:    network.Name,
			network: addr.String(),
			driver:  opts.driver,
			status:  network.Status.State,
		})

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
	if opts.Long {
		table.AddField("MACHINE ID", cs.Bold)
	}
	table.AddField("NAME", cs.Bold)
	table.AddField("NETWORK", cs.Bold)
	table.AddField("DRIVER", cs.Bold)
	table.AddField("STATUS", cs.Bold)
	table.EndRow()

	for _, item := range items {
		if opts.Long {
			table.AddField(item.id, nil)
		}
		table.AddField(item.name, nil)
		table.AddField(item.network, nil)
		table.AddField(item.driver, nil)
		table.AddField(item.status.String(), nil)
		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

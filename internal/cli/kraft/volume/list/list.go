// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package list

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	volumeapi "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/volume"
)

type List struct {
	driver string
	Long   bool   `long:"long" short:"l" usage:"Show more information"`
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
}

type colorFunc func(string) string

var (
	VolumeStateColor = map[volumeapi.VolumeState]colorFunc{
		volumeapi.VolumeStateBound:   iostreams.Green,
		volumeapi.VolumeStateLost:    iostreams.Red,
		volumeapi.VolumeStatePending: iostreams.Blue,
	}
	VolumeStateColorNil = map[volumeapi.VolumeState]colorFunc{
		volumeapi.VolumeStateBound:   nil,
		volumeapi.VolumeStateLost:    nil,
		volumeapi.VolumeStatePending: nil,
	}
)

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&List{}, cobra.Command{
		Short:   "List machine volumes",
		Use:     "ls [FLAGS]",
		Aliases: []string{"list"},
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "volume",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *List) Pre(cmd *cobra.Command, _ []string) error {
	opts.driver = cmd.Flag("driver").Value.String()
	return nil
}

func (opts *List) Run(ctx context.Context, args []string) error {
	var err error

	strategy, ok := volume.Strategies()[opts.driver]
	if !ok {
		return fmt.Errorf("unsupported volume driver strategy: %s", opts.driver)
	}

	controller, err := strategy.NewVolumeV1alpha1(ctx)
	if err != nil {
		return err
	}

	volumes, err := controller.List(ctx, &volumeapi.VolumeList{})
	if err != nil {
		return err
	}

	type volTableEntry struct {
		driver string
		id     string
		name   string
		source string
		status volumeapi.VolumeState
	}

	var items []volTableEntry

	for _, volume := range volumes.Items {
		items = append(items, volTableEntry{
			driver: opts.driver,
			id:     string(volume.UID),
			name:   volume.Name,
			source: volume.Spec.Source,
			status: volume.Status.State,
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

	if config.G[config.KraftKit](ctx).NoColor {
		VolumeStateColor = VolumeStateColorNil
	}

	// Header row
	table.AddField("DRIVER", cs.Bold)
	table.AddField("VOLUME NAME", cs.Bold)
	if opts.Long {
		table.AddField("VOLUME ID", cs.Bold)
	}
	table.AddField("STATUS", cs.Bold)
	table.AddField("SOURCE", cs.Bold)
	table.EndRow()

	for _, item := range items {
		table.AddField(item.driver, nil)
		table.AddField(item.name, nil)
		if opts.Long {
			table.AddField(item.id, nil)
		}
		table.AddField(item.status.String(), VolumeStateColor[item.status])
		table.AddField(item.source, nil)
		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package ps

import (
	"fmt"
	"strings"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

type PsOptions struct {
	Architecture string `long:"arch" short:"m" usage:"Filter the list by architecture"`
	Long         bool   `long:"long" short:"l" usage:"Show more information"`
	platform     string
	Quiet        bool   `long:"quiet" short:"q" usage:"Only display machine IDs"`
	ShowAll      bool   `long:"all" short:"a" usage:"Show all machines (default shows just running)"`
	Output       string `long:"output" short:"o" usage:"Set output format" default:"table"`
}

const (
	MemoryMiB = 1024 * 1024
)

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&PsOptions{}, cobra.Command{
		Short: "List running unikernels",
		Use:   "ps [FLAGS]",
		Args:  cobra.MaximumNArgs(0),
		Long:  "List running unikernels",
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "run",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.Flags().VarP(
		cmdfactory.NewEnumFlag[mplatform.Platform](
			mplatform.Platforms(),
			mplatform.Platform("all"),
		),
		"plat",
		"p",
		"Set the platform virtual machine monitor driver.",
	)

	return cmd
}

func (opts *PsOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.platform = cmd.Flag("plat").Value.String()
	return nil
}

func (opts *PsOptions) Run(cmd *cobra.Command, _ []string) error {
	var err error

	type psTable struct {
		id      string
		name    string
		kernel  string
		args    string
		created string
		status  machineapi.MachineState
		mem     string
		ports   string
		arch    string
		plat    string
		ips     []string
	}

	var items []psTable

	ctx := cmd.Context()
	platform := mplatform.PlatformUnknown
	var controller machineapi.MachineService

	if opts.platform == "all" {
		controller, err = mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	} else {
		if opts.platform == "" || opts.platform == "auto" {
			platform, _, err = mplatform.Detect(ctx)
			if err != nil {
				return err
			}
		} else {
			var ok bool
			platform, ok = mplatform.PlatformsByName()[opts.platform]
			if !ok {
				return fmt.Errorf("unknown platform driver: %s", opts.platform)
			}
		}

		strategy, ok := mplatform.Strategies()[platform]
		if !ok {
			return fmt.Errorf("unsupported platform driver: %s (contributions welcome!)", platform.String())
		}

		controller, err = strategy.NewMachineV1alpha1(ctx)
	}
	if err != nil {
		return err
	}

	machines, err := controller.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return err
	}

	for _, machine := range machines.Items {
		entry := psTable{
			id:      string(machine.UID),
			name:    machine.Name,
			args:    strings.Join(machine.Spec.ApplicationArgs, " "),
			kernel:  machine.Spec.Kernel,
			status:  machine.Status.State,
			mem:     fmt.Sprintf("%dMiB", machine.Spec.Resources.Requests.Memory().Value()/MemoryMiB),
			created: humanize.Time(machine.ObjectMeta.CreationTimestamp.Time),
			ports:   machine.Spec.Ports.String(),
			arch:    machine.Spec.Architecture,
			plat:    machine.Spec.Platform,
			ips:     []string{},
		}

		for _, net := range machine.Spec.Networks {
			for _, iface := range net.Interfaces {
				entry.ips = append(entry.ips, iface.Spec.IP)
			}
		}

		items = append(items, entry)
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
	table.AddField("KERNEL", cs.Bold)
	table.AddField("ARGS", cs.Bold)
	table.AddField("CREATED", cs.Bold)
	table.AddField("STATUS", cs.Bold)
	table.AddField("MEM", cs.Bold)
	if opts.Long {
		table.AddField("PORTS", cs.Bold)
		table.AddField("IP", cs.Bold)
		table.AddField("ARCH", cs.Bold)
	}
	table.AddField("PLAT", cs.Bold)
	table.EndRow()

	for _, item := range items {
		if opts.Long {
			table.AddField(item.id, nil)
		}
		table.AddField(item.name, nil)
		table.AddField(item.kernel, nil)
		table.AddField(item.args, nil)
		table.AddField(item.created, nil)
		table.AddField(item.status.String(), nil)
		table.AddField(item.mem, nil)
		if opts.Long {
			table.AddField(item.ports, nil)
			table.AddField(strings.Join(item.ips, ","), nil)
			table.AddField(item.arch, nil)
			table.AddField(item.plat, nil)
		} else {
			table.AddField(fmt.Sprintf("%s/%s", item.plat, item.arch), nil)
		}
		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

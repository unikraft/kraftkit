// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package ps

import (
	"context"
	"fmt"
	"strings"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

type PsOptions struct {
	Architecture string `long:"arch" short:"m" usage:"Filter the list by architecture"`
	Long         bool   `long:"long" short:"l" usage:"Show more information"`
	platform     string
	Quiet        bool   `long:"quiet" short:"q" usage:"Only display machine IDs"`
	ShowAll      bool   `long:"all" short:"a" usage:"Show all machines (default shows just running)"`
	Output       string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,full" default:"table"`
}

const (
	MemoryMiB = 1024 * 1024
)

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PsOptions{}, cobra.Command{
		Short:   "List running unikernels",
		Use:     "ps [FLAGS]",
		Args:    cobra.MaximumNArgs(0),
		Aliases: []string{},
		Long:    "List running unikernels",
		Example: heredoc.Doc(`
			# List all running unikernels
			$ kraft ps

			# List all unikernels
			$ kraft ps --all

			# List all running unikernels with more information
			$ kraft ps --long
		`),
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

type PsEntry struct {
	ID      string
	Name    string
	Kernel  string
	Args    string
	Created string
	Status  machineapi.MachineState
	Mem     string
	Ports   string
	Arch    string
	Plat    string
	IPs     []string
}

func (opts *PsOptions) Run(ctx context.Context, _ []string) error {
	items, err := opts.PsTable(ctx)
	if err != nil {
		return err
	}

	return opts.PrintPsTable(ctx, items)
}

func (opts *PsOptions) PsTable(ctx context.Context) ([]PsEntry, error) {
	var err error
	var items []PsEntry

	platform := mplatform.PlatformUnknown
	var controller machineapi.MachineService

	if opts.platform == "all" {
		controller, err = mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	} else {
		if opts.platform == "" || opts.platform == "auto" {
			platform, _, err = mplatform.Detect(ctx)
			if err != nil {
				return nil, err
			}
		} else {
			var ok bool
			platform, ok = mplatform.PlatformsByName()[opts.platform]
			if !ok {
				return nil, fmt.Errorf("unknown platform driver: %s", opts.platform)
			}
		}

		strategy, ok := mplatform.Strategies()[platform]
		if !ok {
			return nil, fmt.Errorf("unsupported platform driver: %s (contributions welcome!)", platform.String())
		}

		controller, err = strategy.NewMachineV1alpha1(ctx)
	}
	if err != nil {
		return nil, err
	}

	machines, err := controller.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return nil, err
	}

	for _, machine := range machines.Items {
		if !opts.ShowAll && machine.Status.State != machineapi.MachineStateRunning {
			continue
		}
		entry := PsEntry{
			ID:      string(machine.UID),
			Name:    machine.Name,
			Args:    strings.Join(machine.Spec.ApplicationArgs, " "),
			Kernel:  machine.Spec.Kernel,
			Status:  machine.Status.State,
			Mem:     fmt.Sprintf("%dMiB", machine.Spec.Resources.Requests.Memory().Value()/MemoryMiB),
			Created: humanize.Time(machine.ObjectMeta.CreationTimestamp.Time),
			Ports:   machine.Spec.Ports.String(),
			Arch:    machine.Spec.Architecture,
			Plat:    machine.Spec.Platform,
			IPs:     []string{},
		}

		for _, net := range machine.Spec.Networks {
			for _, iface := range net.Interfaces {
				entry.IPs = append(entry.IPs, iface.Spec.CIDR)
			}
		}

		items = append(items, entry)
	}

	return items, nil
}

func (opts *PsOptions) PrintPsTable(ctx context.Context, items []PsEntry) error {
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
			table.AddField(item.ID, nil)
		}
		table.AddField(item.Name, nil)
		table.AddField(item.Kernel, nil)
		table.AddField(item.Args, nil)
		table.AddField(item.Created, nil)
		table.AddField(item.Status.String(), nil)
		table.AddField(item.Mem, nil)
		if opts.Long {
			table.AddField(item.Ports, nil)
			table.AddField(strings.Join(item.IPs, ","), nil)
			table.AddField(item.Arch, nil)
			table.AddField(item.Plat, nil)
		} else {
			table.AddField(fmt.Sprintf("%s/%s", item.Plat, item.Arch), nil)
		}
		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

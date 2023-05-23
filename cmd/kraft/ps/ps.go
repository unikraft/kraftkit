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
	"kraftkit.sh/internal/set"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

type Ps struct {
	Architecture string `long:"arch" short:"m" usage:"Filter the list by architecture"`
	Long         bool   `long:"long" short:"l" usage:"Show more information"`
	platform     string
	Quiet        bool `long:"quiet" short:"q" usage:"Only display machine IDs"`
	ShowAll      bool `long:"all" short:"a" usage:"Show all machines (default shows just running)"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Ps{}, cobra.Command{
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
		cmdfactory.NewEnumFlag(set.NewStringSet(mplatform.DriverNames()...).Add("auto").ToSlice(), "auto"),
		"plat",
		"p",
		"Set the platform virtual machine monitor driver.",
	)

	return cmd
}

func (opts *Ps) Pre(cmd *cobra.Command, _ []string) error {
	opts.platform = cmd.Flag("plat").Value.String()
	return nil
}

func (opts *Ps) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	platform := mplatform.PlatformUnknown

	if opts.platform == "" || opts.platform == "auto" {
		var mode mplatform.SystemMode
		platform, mode, err = mplatform.Detect(ctx)
		if mode == mplatform.SystemGuest {
			return fmt.Errorf("nested virtualization not supported")
		} else if err != nil {
			return err
		}
	} else {
		var ok bool
		platform, ok = mplatform.Platforms()[opts.platform]
		if !ok {
			return fmt.Errorf("unknown platform driver: %s", opts.platform)
		}
	}

	strategy, ok := mplatform.Strategies()[platform]
	if !ok {
		return fmt.Errorf("unsupported platform driver: %s (contributions welcome!)", platform.String())
	}

	controller, err := strategy.NewMachineV1alpha1(ctx)
	if err != nil {
		return err
	}

	machines, err := controller.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return err
	}

	type psTable struct {
		id      string
		name    string
		kernel  string
		args    string
		created string
		status  machineapi.MachineState
		mem     string
		arch    string
		plat    string
		driver  string
	}

	var items []psTable

	for _, machine := range machines.Items {
		items = append(items, psTable{
			id:      string(machine.UID),
			name:    machine.Name,
			args:    strings.Join(machine.Spec.ApplicationArgs, " "),
			kernel:  machine.Spec.Kernel,
			status:  machine.Status.State,
			mem:     fmt.Sprintf("%dM", machine.Spec.Resources.Requests.Memory().Value()/1000000),
			created: humanize.Time(machine.ObjectMeta.CreationTimestamp.Time),
			arch:    machine.Spec.Architecture,
			plat:    machine.Spec.Platform,
			driver:  platform.String(),
		})
	}

	err = iostreams.G(ctx).StartPager()
	if err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	}

	defer iostreams.G(ctx).StopPager()

	cs := iostreams.G(ctx).ColorScheme()
	table := tableprinter.NewTablePrinter(ctx)

	// Header row
	if opts.Long {
		table.AddField("MACHINE ID", nil, cs.Bold)
	}
	table.AddField("NAME", nil, cs.Bold)
	table.AddField("KERNEL", nil, cs.Bold)
	table.AddField("ARGS", nil, cs.Bold)
	table.AddField("CREATED", nil, cs.Bold)
	table.AddField("STATUS", nil, cs.Bold)
	table.AddField("MEM", nil, cs.Bold)
	if opts.Long {
		table.AddField("ARCH", nil, cs.Bold)
		table.AddField("PLAT", nil, cs.Bold)
		table.AddField("DRIVER", nil, cs.Bold)
	}
	table.EndRow()

	for _, item := range items {
		if opts.Long {
			table.AddField(item.id, nil, nil)
		}
		table.AddField(item.name, nil, nil)
		table.AddField(item.kernel, nil, nil)
		table.AddField(item.args, nil, nil)
		table.AddField(item.created, nil, nil)
		table.AddField(item.status.String(), nil, nil)
		table.AddField(item.mem, nil, nil)
		if opts.Long {
			table.AddField(item.arch, nil, nil)
			table.AddField(item.plat, nil, nil)
			table.AddField(item.driver, nil, nil)
		}
		table.EndRow()
	}

	return table.Render()
}

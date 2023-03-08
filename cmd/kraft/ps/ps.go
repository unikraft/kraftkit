// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package ps

import (
	"fmt"
	"strconv"
	"strings"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine"
	machinedriver "kraftkit.sh/machine/driver"
	machinedriveropts "kraftkit.sh/machine/driveropts"
	"kraftkit.sh/utils"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

type Ps struct {
	Architecture string `long:"arch" short:"m" usage:"Filter the list by architecture"`
	Hypervisor   string
	Long         bool   `long:"long" short:"l" usage:"Show more information"`
	Platform     string `long:"plat" short:"p" usage:"Filter the list by platform"`
	Quiet        bool   `long:"quiet" short:"q" usage:"Only display machine IDs"`
	ShowAll      bool   `long:"all" short:"a" usage:"Show all machines (default shows just running)"`
}

func New() *cobra.Command {
	cmd := cmdfactory.New(&Ps{}, cobra.Command{
		Short: "List running unikernels",
		Use:   "ps [FLAGS]",
		Args:  cobra.MaximumNArgs(0),
		Long:  "List running unikernels",
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "run",
		},
	})

	cmd.Flags().VarP(
		cmdfactory.NewEnumFlag(machinedriver.DriverNames(), "all"),
		"hypervisor",
		"H",
		"Set the hypervisor driver.",
	)

	return cmd
}

func (opts *Ps) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	opts.Hypervisor = cmd.Flag("hypervisor").Value.String()

	var onlyDriverType *machinedriver.DriverType
	if opts.Hypervisor == "all" {
		dt, err := machinedriver.DetectHostHypervisor()
		if err != nil {
			return err
		}
		onlyDriverType = &dt
	} else {
		if !utils.Contains(machinedriver.DriverNames(), opts.Hypervisor) {
			return fmt.Errorf("unknown hypervisor driver: %s", opts.Hypervisor)
		}
	}

	type psTable struct {
		id      machine.MachineID
		image   string
		args    string
		created string
		status  machine.MachineState
		mem     string
		arch    string
		plat    string
		driver  string
	}

	var items []psTable

	store, err := machine.NewMachineStoreFromPath(config.G[config.KraftKit](ctx).RuntimeDir)
	if err != nil {
		return err
	}

	mids, err := store.ListAllMachineConfigs()
	if err != nil {
		return err
	}

	drivers := make(map[machinedriver.DriverType]machinedriver.Driver)

	for mid, mopts := range mids {
		if onlyDriverType != nil && mopts.DriverName != onlyDriverType.String() {
			continue
		}

		driverType := machinedriver.DriverTypeFromName(mopts.DriverName)
		if driverType == machinedriver.UnknownDriver {
			log.G(ctx).Warnf("unknown driver %s for %s", driverType.String(), mid)
			continue
		}

		if _, ok := drivers[driverType]; !ok {
			driver, err := machinedriver.New(driverType,
				machinedriveropts.WithRuntimeDir(config.G[config.KraftKit](ctx).RuntimeDir),
				machinedriveropts.WithMachineStore(store),
			)
			if err != nil {
				return err
			}

			drivers[driverType] = driver
		}

		driver := drivers[driverType]

		state, err := driver.State(ctx, mid)
		if err != nil {
			return err
		}

		if !opts.ShowAll && state != machine.MachineStateRunning {
			continue
		}

		items = append(items, psTable{
			id:      mid,
			args:    strings.Join(mopts.Arguments, " "),
			image:   mopts.Source,
			status:  state,
			mem:     strconv.FormatUint(mopts.MemorySize, 10) + "MB",
			created: humanize.Time(mopts.CreatedAt),
			arch:    mopts.Architecture,
			plat:    mopts.Platform,
			driver:  mopts.DriverName,
		})
	}

	err = iostreams.G(ctx).StartPager()
	if err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	}

	defer iostreams.G(ctx).StopPager()

	cs := iostreams.G(ctx).ColorScheme()
	table := utils.NewTablePrinter(ctx)

	// Header row
	table.AddField("MACHINE ID", nil, cs.Bold)
	table.AddField("IMAGE", nil, cs.Bold)
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
		table.AddField(item.id.ShortString(), nil, nil)
		table.AddField(item.image, nil, nil)
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

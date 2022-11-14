// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package ps

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine"
	machinedriver "kraftkit.sh/machine/driver"
	machinedriveropts "kraftkit.sh/machine/driveropts"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/utils"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

type psOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams

	// Command-line arguments
	ShowAll      bool
	Hypervisor   string
	Architecture string
	Platform     string
	Quiet        bool
	Long         bool
}

func PsCmd(f *cmdfactory.Factory) *cobra.Command {
	cmd, err := cmdutil.NewCmd(f, "ps",
		cmdutil.WithSubcmds(),
	)
	if err != nil {
		panic("could not initialize 'kraft ps' command")
	}

	opts := &psOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	cmd.Short = "List running unikernels"
	cmd.Use = "ps [FLAGS]"
	cmd.Args = cobra.MaximumNArgs(0)
	cmd.Long = heredoc.Doc(`
		List running unikernels`)
	cmd.Example = heredoc.Doc(``)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		opts.Hypervisor = cmd.Flag("hypervisor").Value.String()

		return runPs(opts, args...)
	}

	cmd.Flags().BoolVarP(
		&opts.ShowAll,
		"all", "a",
		false,
		"Show all machines (default shows just running)",
	)

	cmd.Flags().BoolVarP(
		&opts.Quiet,
		"quiet", "q",
		false,
		"Only display machine IDs",
	)

	cmd.Flags().VarP(
		cmdutil.NewEnumFlag(machinedriver.DriverNames(), "all"),
		"hypervisor",
		"H",
		"Set the hypervisor driver.",
	)

	cmd.Flags().StringVarP(
		&opts.Architecture,
		"arch", "m",
		"",
		"Filter the list by architecture",
	)

	cmd.Flags().StringVarP(
		&opts.Platform,
		"plat", "p",
		"",
		"Filter the list by platform",
	)

	cmd.Flags().BoolVarP(
		&opts.Long,
		"long", "l",
		false,
		"Show more information",
	)

	return cmd
}

func runPs(opts *psOptions, args ...string) error {
	var err error

	plog, err := opts.Logger()
	if err != nil {
		return err
	}

	cfgm, err := opts.ConfigManager()
	if err != nil {
		return err
	}

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

	store, err := machine.NewMachineStoreFromPath(cfgm.Config.RuntimeDir)
	if err != nil {
		return err
	}

	mids, err := store.ListAllMachineConfigs()
	if err != nil {
		return err
	}

	ctx := context.Background()

	drivers := make(map[machinedriver.DriverType]machinedriver.Driver)

	for mid, mopts := range mids {
		if onlyDriverType != nil && mopts.DriverName != onlyDriverType.String() {
			continue
		}

		driverType := machinedriver.DriverTypeFromName(mopts.DriverName)
		if driverType == machinedriver.UnknownDriver {
			plog.Warnf("unknown driver %s for %s", driverType.String(), mid)
			continue
		}

		if _, ok := drivers[driverType]; !ok {
			driver, err := machinedriver.New(driverType,
				machinedriveropts.WithRuntimeDir(cfgm.Config.RuntimeDir),
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

	err = opts.IO.StartPager()
	if err != nil {
		plog.Errorf("error starting pager: %v", err)
	}

	defer opts.IO.StopPager()

	cs := opts.IO.ColorScheme()
	table := utils.NewTablePrinter(opts.IO)

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

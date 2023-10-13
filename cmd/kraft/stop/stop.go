// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package stop

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"
)

type Stop struct {
	All      bool `long:"all" usage:"Remove all machines"`
	platform string
}

func New(cfg *config.ConfigManager[config.KraftKit]) *cobra.Command {
	cmd, err := cmdfactory.New(&Stop{}, cobra.Command{
		Short: "Stop one or more running unikernels",
		Use:   "stop [FLAGS] MACHINE [MACHINE [...]]",
		Long: heredoc.Doc(`
			Stop one or more running unikernels`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "run",
		},
	}, cfg)
	if err != nil {
		panic(err)
	}

	cmd.Flags().VarP(
		cmdfactory.NewEnumFlag(set.NewStringSet(mplatform.DriverNames()...).Add("auto").ToSlice(), "auto"),
		"plat",
		"p",
		"Set the platform virtual machine monitor driver.  Set to 'auto' to detect the guest's platform and 'host' to use the host platform.",
	)

	return cmd
}

func (opts *Stop) Pre(cmd *cobra.Command, args []string, cfg *config.ConfigManager[config.KraftKit]) error {
	if len(args) == 0 && !opts.All {
		return fmt.Errorf("please supply a machine ID or name or use the --all flag")
	}

	opts.platform = cmd.Flag("plat").Value.String()
	return nil
}

func (opts *Stop) Run(cmd *cobra.Command, args []string, cfgMgr *config.ConfigManager[config.KraftKit]) error {
	if len(args) == 0 && !opts.All {
		return fmt.Errorf("please supply a machine ID or name or use the --all flag")
	}

	var err error

	ctx := cmd.Context()
	platform := mplatform.PlatformUnknown
	var controller machineapi.MachineService

	if opts.All || opts.platform == "auto" {
		controller, err = mplatform.NewMachineV1alpha1ServiceIterator(ctx, cfgMgr.Config)
	} else {
		if opts.platform == "host" {
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

		controller, err = strategy.NewMachineV1alpha1(ctx, cfgMgr.Config)
	}
	if err != nil {
		return err
	}

	machines, err := controller.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return err
	}

	var stop []machineapi.Machine

	for _, machine := range machines.Items {
		if opts.All {
			stop = append(stop, machine)
			continue
		}

		if args[0] == machine.Name || args[0] == string(machine.UID) {
			stop = append(stop, machine)
		}
	}

	if len(stop) == 0 {
		return fmt.Errorf("machine(s) not found")
	}

	for _, machine := range stop {
		if machine.Status.State == machineapi.MachineStateExited {
			continue
		} else if _, err := controller.Stop(ctx, &machine); err != nil {
			log.G(ctx).Errorf("could not stop machine %s: %v", machine.Name, err)
		} else {
			fmt.Fprintln(iostreams.G(ctx).Out, machine.Name)
		}
	}

	return nil
}

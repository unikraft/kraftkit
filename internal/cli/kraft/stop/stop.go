// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package stop

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"
)

type StopOptions struct {
	All      bool   `long:"all" usage:"Remove all machines"`
	Platform string `noattribute:"true"`
}

// Stop a local Unikraft virtual machine.
func Stop(ctx context.Context, opts *StopOptions, args ...string) error {
	if opts == nil {
		opts = &StopOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StopOptions{}, cobra.Command{
		Short:   "Stop one or more running unikernels",
		Use:     "stop [FLAGS] MACHINE [MACHINE [...]]",
		Aliases: []string{},
		Long: heredoc.Doc(`
			Stop one or more running unikernels
		`),
		Example: heredoc.Doc(`
			# Stop a running unikernel
			$ kraft stop my-machine
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
			mplatform.Platform("auto"),
		),
		"plat",
		"p",
		"Set the platform virtual machine monitor driver.  Set to 'auto' to detect the guest's platform and 'host' to use the host platform.",
	)

	return cmd
}

func (opts *StopOptions) Pre(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && !opts.All {
		return fmt.Errorf("please supply a machine ID or name or use the --all flag")
	}

	opts.Platform = cmd.Flag("plat").Value.String()
	return nil
}

func (opts *StopOptions) Run(ctx context.Context, args []string) error {
	if len(args) == 0 && !opts.All {
		return fmt.Errorf("please supply a machine ID or name or use the --all flag")
	}

	var err error

	platform := mplatform.PlatformUnknown
	var controller machineapi.MachineService

	if opts.All || opts.Platform == "auto" {
		controller, err = mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	} else {
		if opts.Platform == "host" {
			platform, _, err = mplatform.Detect(ctx)
			if err != nil {
				return err
			}
		} else {
			var ok bool
			platform, ok = mplatform.PlatformsByName()[opts.Platform]
			if !ok {
				return fmt.Errorf("unknown platform driver: %s", opts.Platform)
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

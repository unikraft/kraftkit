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
	"kraftkit.sh/internal/set"
	"kraftkit.sh/internal/waitgroup"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"
)

type Stop struct {
	All      bool `long:"all" usage:"Remove all machines"`
	platform string
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Stop{}, cobra.Command{
		Short: "Stop one or more running unikernels",
		Use:   "stop [FLAGS] MACHINE [MACHINE [...]]",
		Args:  cobra.MaximumNArgs(1),
		Long: heredoc.Doc(`
			Stop one or more running unikernels`),
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

var observations = waitgroup.WaitGroup[*machineapi.Machine]{}

func (opts *Stop) Pre(cmd *cobra.Command, _ []string) error {
	opts.platform = cmd.Flag("plat").Value.String()
	return nil
}

func (opts *Stop) Run(cmd *cobra.Command, args []string) error {
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

	var remove []*machineapi.Machine

	for _, machine := range machines.Items {
		if len(args) == 0 && opts.All {
			remove = append(remove, &machine)
			continue
		}

		if args[0] == machine.Name || args[0] == string(machine.UID) {
			remove = append(remove, &machine)
		}
	}

	for _, machine := range remove {
		machine := machine // loop closure

		if observations.Contains(machine) {
			continue
		}

		observations.Add(machine)

		go func() {
			observations.Add(machine)

			log.G(ctx).Infof("stopping %s", machine.Name)

			if _, err := controller.Stop(ctx, machine); err != nil {
				log.G(ctx).Errorf("could not stop machine %s: %v", machine.Name, err)
			} else {
				log.G(ctx).Infof("stopped %s", machine.Name)
			}

			observations.Done(machine)
		}()
	}

	observations.Wait()

	return nil
}

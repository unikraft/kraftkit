// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package logs

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"
)

type Logs struct {
	platform string
	Follow   bool `long:"follow" short:"f" usage:"Follow log output"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Logs{}, cobra.Command{
		Short:   "Fetch the logs of a unikernel.",
		Use:     "logs [FLAGS] MACHINE",
		Args:    cobra.MaximumNArgs(1),
		GroupID: "run",
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

func (opts *Logs) Pre(cmd *cobra.Command, _ []string) error {
	opts.platform = cmd.Flag("plat").Value.String()
	return nil
}

func (opts *Logs) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	platform := mplatform.PlatformUnknown
	var controller machineapi.MachineService

	if len(args) == 0 {
		return fmt.Errorf("must specify a machine to fetch logs for")
	}

	if opts.platform == "auto" {
		controller, err = mplatform.NewMachineV1alpha1ServiceIterator(ctx)
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

		controller, err = strategy.NewMachineV1alpha1(ctx)
	}
	if err != nil {
		return err
	}

	machines, err := controller.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return err
	}

	var machine *machineapi.Machine

	for _, candidate := range machines.Items {
		if args[0] == candidate.Name {
			machine = &candidate
			break
		} else if string(candidate.UID) == args[0] {
			machine = &candidate
			break
		}
	}

	if machine == nil {
		return fmt.Errorf("could not find instance %s", args[0])
	}

	if opts.Follow && machine.Status.State == machineapi.MachineStateRunning {
		ctx, cancel := context.WithCancel(ctx)

		go func() {
			events, errs, err := controller.Watch(ctx, machine)
			if err != nil {
				cancel()
				log.G(ctx).Errorf("could not listen for machine updates: %v", err)
				return
			}

		loop:
			for {
				// Wait on either channel
				select {
				case status := <-events:
					switch status.Status.State {
					case machineapi.MachineStateExited, machineapi.MachineStateFailed:
						break loop
					}

				case err := <-errs:
					log.G(ctx).Errorf("received event error: %v", err)
					break loop

				case <-ctx.Done():
					break loop
				}
			}
		}()

		logs, errs, err := controller.Logs(ctx, machine)
		if err != nil {
			cancel()
			return err
		}

	loop:
		for {
			// Wait on either channel
			select {
			case line := <-logs:
				fmt.Fprint(iostreams.G(ctx).Out, line)

			case err := <-errs:
				log.G(ctx).Errorf("received event error: %v", err)
				break loop

			case <-ctx.Done():
				break loop
			}
		}
	} else {
		fd, err := os.Open(machine.Status.LogFile)
		if err != nil {
			return err
		}

		if _, err := io.Copy(iostreams.G(ctx).Out, fd); err != nil {
			return err
		}
	}

	return nil
}

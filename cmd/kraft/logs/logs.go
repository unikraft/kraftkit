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

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine"
	machinedriver "kraftkit.sh/machine/driver"
	machinedriveropts "kraftkit.sh/machine/driveropts"
)

type Logs struct {
	Follow bool `long:"follow" short:"f" usage:"Follow log output"`
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
		cmdfactory.NewEnumFlag(machinedriver.DriverNames(), "auto"),
		"hypervisor",
		"H",
		"Set the hypervisor machine driver.",
	)

	return cmd
}

func (opts *Logs) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()

	debug := log.Levels()[config.G[config.KraftKit](ctx).Log.Level] >= logrus.DebugLevel
	store, err := machine.NewMachineStoreFromPath(config.G[config.KraftKit](ctx).RuntimeDir)
	if err != nil {
		return fmt.Errorf("could not access machine store: %v", err)
	}

	mcfgs, err := store.ListAllMachineConfigs()
	if err != nil {
		return err
	}

	var mcfg *machine.MachineConfig

	for _, candidate := range mcfgs {
		if machine.MachineName(args[0]) == candidate.Name {
			mcfg = &candidate
			break
		} else if candidate.ID.Short().String() == args[0] {
			mcfg = &candidate
			break
		} else if candidate.ID.String() == args[0] {
			mcfg = &candidate
			break
		}
	}

	if mcfg == nil {
		return fmt.Errorf("could not find instance %s", args[0])
	}

	driver, err := machinedriver.New(machinedriver.DriverTypeFromName(mcfg.DriverName),
		machinedriveropts.WithBackground(false),
		machinedriveropts.WithRuntimeDir(config.G[config.KraftKit](ctx).RuntimeDir),
		machinedriveropts.WithMachineStore(store),
		machinedriveropts.WithDebug(debug),
		machinedriveropts.WithExecOptions(
			exec.WithStdout(os.Stdout),
			exec.WithStderr(os.Stderr),
		),
	)
	if err != nil {
		return err
	}

	mid := mcfg.ID

	// Skip checking the error, if we receive an error, we will not tail the logs.
	state, _ := driver.State(ctx, mid)

	if opts.Follow && state == machine.MachineStateRunning {
		ctx, cancel := context.WithCancel(ctx)

		go func() {
			events, errs, err := driver.ListenStatusUpdate(ctx, mid)
			if err != nil {
				log.G(ctx).Errorf("could not listen for machine updates: %v", err)
				return
			}

		loop:
			for {
				// Wait on either channel
				select {
				case status := <-events:
					if err := store.SaveMachineState(mid, status); err != nil {
						log.G(ctx).Errorf("could not save machine state: %v", err)
						return
					}

					switch status {
					case machine.MachineStateExited, machine.MachineStateDead:
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

		if tErr := driver.TailWriter(ctx, mid, iostreams.G(ctx).Out); tErr != nil {
			err = fmt.Errorf("%w. Additionally, while tailing writer: %w", err, tErr)
		}
		cancel()
		return err
	} else {
		fd, err := os.Open(mcfg.LogFile)
		if err != nil {
			return err
		}

		if _, err := io.Copy(iostreams.G(ctx).Out, fd); err != nil {
			return err
		}
	}

	return nil
}

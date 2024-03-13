// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package logs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/waitgroup"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	mplatform "kraftkit.sh/machine/platform"
)

type LogOptions struct {
	Follow   bool   `long:"follow" short:"f" usage:"Follow log output"`
	Platform string `noattribute:"true"`
	NoPrefix bool   `long:"no-prefix" usage:"When logging multiple machines, do not prefix each log line with the name"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&LogOptions{}, cobra.Command{
		Short:   "Fetch the logs of a unikernel",
		Use:     "logs [FLAGS] MACHINE",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"log"},
		Long: heredoc.Doc(`
			Fetch the logs of a unikernel.
		`),
		Example: heredoc.Doc(`
			# Fetch the logs of a unikernel
			$ kraft logs my-machine

			# Fetch the logs of a unikernel and follow the output
			$ kraft logs --follow my-machine

			# Fetch the logs of multiple unikernels and follow the output
			$ kraft logs --follow my-machine1 my-machine2
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

func (opts *LogOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.Platform = cmd.Flag("plat").Value.String()

	return nil
}

func (opts *LogOptions) Run(ctx context.Context, args []string) error {
	var err error

	platform := mplatform.PlatformUnknown
	var controller machineapi.MachineService

	if opts.Platform == "auto" {
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

	loggedMachines := []*machineapi.Machine{}

	// Although this looks duplicated, it allows us to check whether all arguments
	// are a valid machine while also not having duplicated logging in case of
	// multiple equal arguments (or both the name and UID).
	for _, candidate := range machines.Items {
		for _, arg := range args {
			if arg == candidate.Name || arg == string(candidate.UID) {
				loggedMachines = append(loggedMachines, &candidate)
				break
			}
		}
	}

	for _, arg := range args {
		found := false
		for _, machine := range loggedMachines {
			if arg == machine.Name || arg == string(machine.UID) {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("could not find instance %s", arg)
		}
	}

	longestName := 0

	if len(loggedMachines) > 1 && !opts.NoPrefix {
		for _, machine := range loggedMachines {
			if len(machine.Name) > longestName {
				longestName = len(machine.Name)
			}
		}
	} else {
		opts.NoPrefix = true
	}

	var errGroup []error
	observations := waitgroup.WaitGroup[*machineapi.Machine]{}

	for _, machine := range loggedMachines {
		prefix := ""
		if !opts.NoPrefix {
			prefix = machine.Name + strings.Repeat(" ", longestName-len(machine.Name))
		}
		consumer, err := NewColorfulConsumer(iostreams.G(ctx), !config.G[config.KraftKit](ctx).NoColor, prefix)
		if err != nil {
			errGroup = append(errGroup, err)
		}
		if opts.Follow && machine.Status.State == machineapi.MachineStateRunning {
			observations.Add(machine)
			go func(machine *machineapi.Machine) {
				defer func() {
					observations.Done(machine)
				}()

				if err = FollowLogs(ctx, machine, controller, consumer); err != nil {
					errGroup = append(errGroup, err)
					return
				}
			}(machine)
		} else {
			fd, err := os.Open(machine.Status.LogFile)
			if err != nil {
				return err
			}
			defer fd.Close()

			if prefix == "" {
				if _, err := io.Copy(iostreams.G(ctx).Out, fd); err != nil {
					errGroup = append(errGroup, err)
				}
			} else {
				scanner := bufio.NewScanner(fd)
				for scanner.Scan() {
					if err := consumer.Consume(scanner.Text() + "\n"); err != nil {
						errGroup = append(errGroup, err)
					}
				}
			}
		}
	}

	observations.Wait()

	return errors.Join(errGroup...)
}

// FollowLogs tracks the logs generated by a machine and prints them to the context out stream.
func FollowLogs(ctx context.Context, machine *machineapi.Machine, controller machineapi.MachineService, consumer LogConsumer) error {
	ctx, cancel := context.WithCancel(ctx)

	var exitErr error
	var eof bool

	go func() {
		events, errs, err := controller.Watch(ctx, machine)
		if err != nil {
			if eof {
				cancel()
				return
			}

			// There is a chance that the kernel has booted and exited faster than an
			// event stream can be initialized and interpreted by KraftKit.  This
			// typically happens on M-series processors from Apple.  In the event of
			// an error, first statically check the state of the machine.  If the
			// machine has exited, we can simply return early such that the logs can
			// be output appropriately.
			machine, getMachineErr := controller.Get(ctx, machine)
			if err != nil {
				cancel()
				err = fmt.Errorf("getting the machine information: %w: %w", getMachineErr, err)
			}
			if machine.Status.State == machineapi.MachineStateExited {
				// Calling cancel() in every execution path of this Go routine following
				// the static detection of a preemptive exit state (since the event
				// stream is no longer available) would prevent the tailing of the now
				// finite logs.  A return here without calling cancel() guarantees a
				// graceful exit and the output of said logs.
				return
			}

			cancel()
			exitErr = fmt.Errorf("listening to machine events: %w", err)
			return
		}

	loop:
		for {
			// Wait on either channel
			select {
			case status := <-events:
				switch status.Status.State {
				case machineapi.MachineStateErrored:
					exitErr = fmt.Errorf("machine fatally exited")
					cancel()
					break loop

				case machineapi.MachineStateExited, machineapi.MachineStateFailed:
					cancel()
					break loop
				}

			case err := <-errs:
				log.G(ctx).Errorf("received event error: %v", err)
				exitErr = err
				cancel()
				break loop

			case <-ctx.Done():
				break loop
			}
		}
	}()

	logs, errs, err := controller.Logs(ctx, machine)
	if err != nil {
		cancel()
		return fmt.Errorf("accessing logs: %w", err)
	}

loop:
	for {
		// Wait on either channel
		select {
		case line := <-logs:
			if err := consumer.Consume(line); err != nil {
				log.G(ctx).Errorf("could not consume log line: %v", err)
				return err
			}

		case err := <-errs:
			eof = true
			if !errors.Is(err, io.EOF) {
				log.G(ctx).Errorf("received event error: %v", err)
				return fmt.Errorf("event: %w", err)
			}

		case <-ctx.Done():
			break loop
		}
	}

	return exitErr
}

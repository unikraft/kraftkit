// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package events

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/internal/waitgroup"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/platform"
	mplatform "kraftkit.sh/machine/platform"
	"kraftkit.sh/machine/qemu/qmp"
)

type Events struct {
	platform     string
	Granularity  time.Duration `long:"poll-granularity" short:"g" usage:"How often the machine store and state should polled"`
	QuitTogether bool          `long:"quit-together" short:"q" usage:"Exit event loop when machine exits"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Events{}, cobra.Command{
		Short:   "Follow the events of a unikernel",
		Hidden:  true,
		Use:     "events [FLAGS] [MACHINE ID]",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"event", "e"},
		Long: heredoc.Doc(`
			Follow the events of a unikernel`),
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

func (opts *Events) Pre(cmd *cobra.Command, _ []string) error {
	opts.platform = cmd.Flag("plat").Value.String()

	opts.platform = platform.PlatformByName(opts.platform).String()

	return nil
}

func (opts *Events) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx, cancel := context.WithCancel(cmd.Context())
	platform := mplatform.PlatformUnknown

	if opts.platform == "" || opts.platform == "auto" {
		platform, _, err = mplatform.Detect(ctx)
		if err != nil {
			cancel()
			return err
		}
	} else {
		var ok bool
		platform, ok = mplatform.PlatformsByName()[opts.platform]
		if !ok {
			cancel()
			return fmt.Errorf("unknown platform driver: %s", opts.platform)
		}
	}

	strategy, ok := mplatform.Strategies()[platform]
	if !ok {
		cancel()
		return fmt.Errorf("unsupported platform driver: %s (contributions welcome!)", platform.String())
	}

	controller, err := strategy.NewMachineV1alpha1(ctx)
	if err != nil {
		cancel()
		return err
	}

	var pidfile *os.File

	// Check if a pid has already been enabled
	if _, err := os.Stat(config.G[config.KraftKit](ctx).EventsPidFile); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(config.G[config.KraftKit](ctx).EventsPidFile), 0o775); err != nil {
			cancel()
			return err
		}

		pidfile, err = os.OpenFile(config.G[config.KraftKit](ctx).EventsPidFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666)
		if err != nil {
			cancel()
			return fmt.Errorf("could not create pidfile: %v", err)
		}

		defer func() {
			_ = pidfile.Close()

			log.G(ctx).Info("removing pid file")
			if err := os.Remove(config.G[config.KraftKit](ctx).EventsPidFile); err != nil {
				log.G(ctx).Errorf("could not remove pid file: %v", err)
			}
		}()

		if _, err := pidfile.Write([]byte(fmt.Sprintf("%d", os.Getpid()))); err != nil {
			cancel()
			return fmt.Errorf("failed to write PID file: %w", err)
		}

		if err := pidfile.Sync(); err != nil {
			cancel()
			return fmt.Errorf("could not sync pid file: %v", err)
		}
	}

	// Handle Ctrl+C of the event monitor
	ctrlc := make(chan os.Signal, 1)
	signal.Notify(ctrlc, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctrlc // wait for Ctrl+C
		cancel()
	}()

	// TODO: Should we throw an error here if a process file already exists?  We
	// use a pid file for `kraft run` to continuously monitor running machines.

	// Actively seek for machines whose events we wish to monitor.  The thread
	// will continuously read from the machine store which can be updated
	// elsewhere and acts as the source-of-truth for VMs which are being
	// instantiated by KraftKit.  The thread dies if there is nothing in the store
	// and the `--quit-together` flag is set.
seek:
	for {
		select {
		case <-ctx.Done():
			break seek
		default:
		}

		machines, err := controller.List(ctx, &machineapi.MachineList{})
		if err != nil {
			return fmt.Errorf("could not list machines: %v", err)
		}

		for _, machine := range machines.Items {
			if len(args) == 0 || (args[0] == string(machine.UID) || args[0] == machine.Name) {
				switch machine.Status.State {
				case machineapi.MachineStateFailed,
					machineapi.MachineStateExited,
					machineapi.MachineStateUnknown:
					if opts.QuitTogether {
						continue
					}
				default:
				}

				observations.Add(&machine)
			}
		}

		if len(observations.Items()) == 0 && opts.QuitTogether {
			cancel()
			break seek
		}

		for _, machine := range observations.Items() {
			machine := machine // loop closure

			go func() {
				events, errs, err := controller.Watch(ctx, machine)
				if err != nil {
					log.G(ctx).Debugf("could not listen for status updates for %s: %v", machine.Name, err)
					return
				}

				for {
					// Wait on either channel
					select {
					case machine := <-events:
						log.G(ctx).Infof("%s : %s", machine.Name, machine.Status.State.String())
						switch machine.Status.State {
						case machineapi.MachineStateExited, machineapi.MachineStateFailed:
							observations.Done(machine)
							return
						}

					case err := <-errs:
						if !errors.Is(err, qmp.ErrAcceptedNonEvent) {
							log.G(ctx).Errorf("%v", err)
						}
						observations.Done(machine)

					case <-ctx.Done():
						observations.Done(machine)
						return
					}
				}
			}()
		}

		time.Sleep(time.Second * opts.Granularity)
	}

	observations.Wait()

	return nil
}

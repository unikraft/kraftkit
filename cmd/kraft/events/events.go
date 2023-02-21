// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package events

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/machine"
	machinedriver "kraftkit.sh/machine/driver"
	"kraftkit.sh/machine/driveropts"
	"kraftkit.sh/machine/qemu/qmp"
)

type Events struct {
	Granularity  time.Duration `long:"poll-granularity" short:"g" usage:"How often the machine store and state should polled"`
	QuitTogether bool          `long:"quit-together" short:"q" usage:"Exit event loop when machine exits"`
}

func New() *cobra.Command {
	return cmdfactory.New(&Events{}, cobra.Command{
		Short:   "Follow the events of a unikernel",
		Hidden:  true,
		Use:     "events [FLAGS] [MACHINE ID]",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"event", "e"},
		Long: heredoc.Doc(`
			Follow the events of a unikernel`),
		Annotations: map[string]string{
			"help:group": "run",
		},
	})
}

type machineWaitGroup struct {
	lock sync.RWMutex
	mids []machine.MachineID
}

func (mwg *machineWaitGroup) Add(mid machine.MachineID) {
	mwg.lock.Lock()
	defer mwg.lock.Unlock()

	if mwg.Contains(mid) {
		return
	}

	mwg.mids = append(mwg.mids, mid)
}

func (mwg *machineWaitGroup) Done(needle machine.MachineID) {
	mwg.lock.Lock()
	defer mwg.lock.Unlock()

	if !mwg.Contains(needle) {
		return
	}

	for i, mid := range mwg.mids {
		if mid == needle {
			mwg.mids = append(mwg.mids[:i], mwg.mids[i+1:]...)
			return
		}
	}
}

func (mwg *machineWaitGroup) Wait() {
	for {
		if len(mwg.mids) == 0 {
			break
		}
	}
}

func (mwg *machineWaitGroup) Contains(needle machine.MachineID) bool {
	for _, mid := range mwg.mids {
		if mid == needle {
			return true
		}
	}

	return false
}

var (
	observations = machineWaitGroup{}
	drivers      = make(map[machinedriver.DriverType]machinedriver.Driver)
)

func (opts *Events) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx, cancel := context.WithCancel(cmd.Context())
	store, err := machine.NewMachineStoreFromPath(config.G[config.KraftKit](ctx).RuntimeDir)
	if err != nil {
		cancel()
		return fmt.Errorf("could not access machine store: %v", err)
	}

	var pidfile *os.File

	// Check if a pid has already been enabled
	if _, err := os.Stat(config.G[config.KraftKit](ctx).EventsPidFile); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(config.G[config.KraftKit](ctx).EventsPidFile), 0o755); err != nil {
			cancel()
			return err
		}

		pidfile, err = os.OpenFile(config.G[config.KraftKit](ctx).EventsPidFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666)
		if err != nil {
			cancel()
			return fmt.Errorf("could not create pidfile: %v", err)
		}

		defer func() {
			pidfile.Close()

			log.G(ctx).Info("removing pid file")
			if err := os.Remove(config.G[config.KraftKit](ctx).EventsPidFile); err != nil {
				log.G(ctx).Errorf("could not remove pid file: %v", err)
			}
		}()

		pidfile.Write([]byte(fmt.Sprintf("%d", os.Getpid())))

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

	// TODO: Should we thrown an error here if a process file already exists? We
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

		var mids []machine.MachineID
		allMids, err := store.ListAllMachineIDs()
		if err != nil {
			return fmt.Errorf("could not list machines: %v", err)
		}

		if len(args) > 0 {
			for _, mid := range allMids {
				if args[0] == mid.String() || args[0] == mid.ShortString() {
					mids = append(mids, mid)
				}
			}
		} else {
			mids = allMids
		}

		if len(mids) == 0 && opts.QuitTogether {
			break
		}

		for _, mid := range mids {
			mid := mid // loop closure

			state, err := store.LookupMachineState(mid)
			if err != nil {
				log.G(ctx).Errorf("could not look up machine state: %v", err)
				continue
			}

			switch state {
			case machine.MachineStateDead,
				machine.MachineStateExited,
				machine.MachineStateUnknown:
				continue
			default:
			}

			if observations.Contains(mid) {
				continue
			}

			log.G(ctx).Infof("monitoring %s", mid.ShortString())

			var mcfg machine.MachineConfig
			if err := store.LookupMachineConfig(mid, &mcfg); err != nil {
				log.G(ctx).Errorf("could not look up machine config: %v", err)
				continue
			}

			go func() {
				observations.Add(mid)

				if opts.QuitTogether {
					defer observations.Done(mid)
				}

				mcfg := &machine.MachineConfig{}
				if err := store.LookupMachineConfig(mid, mcfg); err != nil {
					log.G(ctx).Errorf("could not look up machine config: %v", err)
					observations.Done(mid)
					return
				}

				driverType := machinedriver.DriverTypeFromName(mcfg.DriverName)

				if _, ok := drivers[driverType]; !ok {
					driver, err := machinedriver.New(driverType,
						driveropts.WithMachineStore(store),
						driveropts.WithRuntimeDir(config.G[config.KraftKit](ctx).RuntimeDir),
					)
					if err != nil {
						log.G(ctx).Errorf("could not instantiate machine driver for %s: %v", mid, err)
						observations.Done(mid)
						return
					}

					drivers[driverType] = driver
				}

				driver := drivers[driverType]

				events, errs, err := driver.ListenStatusUpdate(ctx, mid)
				if err != nil {
					log.G(ctx).Warnf("could not listen for status updates for %s: %v", mid.ShortString(), err)

					// Check the state of the machine using the driver, for a more
					// accurate read
					state, err := driver.State(ctx, mid)
					if err != nil {
						log.G(ctx).Errorf("could not look up machine state: %v", err)
					}

					switch state {
					case machine.MachineStateExited, machine.MachineStateDead:
						if mcfg.DestroyOnExit {
							log.G(ctx).Infof("removing %s...", mid.ShortString())
							if err := driver.Destroy(ctx, mid); err != nil {
								log.G(ctx).Errorf("could not remove machine: %v: ", err)
							}
						}
					}

					observations.Done(mid)
					return
				}

				for {
					// Wait on either channel
					select {
					case state := <-events:
						log.G(ctx).Infof("%s : %s", mid.ShortString(), state.String())
						switch state {
						case machine.MachineStateExited, machine.MachineStateDead:
							if mcfg.DestroyOnExit {
								log.G(ctx).Infof("removing %s...", mid.ShortString())
								if err := driver.Destroy(ctx, mid); err != nil {
									log.G(ctx).Errorf("could not remove machine: %v: ", err)
								}
							}
							observations.Done(mid)
							return
						}

					case err := <-errs:
						if !errors.Is(err, qmp.ErrAcceptedNonEvent) {
							log.G(ctx).Errorf("%v", err)
						}
						observations.Done(mid)

					case <-ctx.Done():
						observations.Done(mid)
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

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

	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine"
	machinedriver "kraftkit.sh/machine/driver"
	"kraftkit.sh/machine/driveropts"
	"kraftkit.sh/machine/qemu/qmp"
	"kraftkit.sh/packmanager"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

type eventsOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams

	// Command-line arguments
	QuitTogether bool
	Granularity  time.Duration
}

func EventsCmd(f *cmdfactory.Factory) *cobra.Command {
	cmd, err := cmdutil.NewCmd(f, "events")
	if err != nil {
		panic("could not initialize 'kraft events' command")
	}

	opts := &eventsOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	cmd.Short = "Follow the events of a unikernel"
	cmd.Hidden = true
	cmd.Use = "events [FLAGS] [MACHINE ID]"
	cmd.Args = cobra.MaximumNArgs(1)
	cmd.Aliases = []string{"event", "e"}
	cmd.Long = heredoc.Doc(`
		Follow the events of a unikernel`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runEvents(opts, args...)
	}

	cmd.Flags().BoolVarP(
		&opts.QuitTogether,
		"quit-together", "q",
		false,
		"Exit event loop when machine exits",
	)

	cmd.Flags().DurationVarP(
		&opts.Granularity,
		"poll-granularity", "g",
		1,
		"How often the machine store and state should polled",
	)

	return cmd
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

func runEvents(opts *eventsOptions, args ...string) error {
	var err error

	plog, err := opts.Logger()
	if err != nil {
		return err
	}

	cfgm, err := opts.ConfigManager()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	store, err := machine.NewMachineStoreFromPath(cfgm.Config.RuntimeDir)
	if err != nil {
		cancel()
		return fmt.Errorf("could not access machine store: %v", err)
	}

	var pidfile *os.File

	// Check if a pid has already been enabled
	if _, err := os.Stat(cfgm.Config.EventsPidFile); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cfgm.Config.EventsPidFile), 0o755); err != nil {
			cancel()
			return err
		}

		pidfile, err = os.OpenFile(cfgm.Config.EventsPidFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666)
		if err != nil {
			cancel()
			return fmt.Errorf("could not create pidfile: %v", err)
		}

		defer func() {
			pidfile.Close()

			if err := os.Remove(cfgm.Config.EventsPidFile); err != nil {
				plog.Errorf("could not remove pid file: %v", err)
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
				plog.Errorf("could not look up machine state: %v", err)
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

			plog.Infof("monitoring %s", mid.ShortString())

			var mcfg machine.MachineConfig
			if err := store.LookupMachineConfig(mid, &mcfg); err != nil {
				plog.Errorf("could not look up machine config: %v", err)
				continue
			}

			go func() {
				observations.Add(mid)

				if opts.QuitTogether {
					defer observations.Done(mid)
				}

				mcfg := &machine.MachineConfig{}
				if err := store.LookupMachineConfig(mid, mcfg); err != nil {
					plog.Errorf("could not look up machine config: %v", err)
					observations.Done(mid)
					return
				}

				driverType := machinedriver.DriverTypeFromName(mcfg.DriverName)

				if _, ok := drivers[driverType]; !ok {
					driver, err := machinedriver.New(driverType,
						driveropts.WithLogger(plog),
						driveropts.WithMachineStore(store),
						driveropts.WithRuntimeDir(cfgm.Config.RuntimeDir),
					)
					if err != nil {
						plog.Errorf("could not instantiate machine driver for %s: %v", mid, err)
						observations.Done(mid)
						return
					}

					drivers[driverType] = driver
				}

				driver := drivers[driverType]

				events, errs, err := driver.ListenStatusUpdate(ctx, mid)
				if err != nil {
					plog.Warnf("could not listen for status updates for %s: %v", mid.ShortString(), err)

					// Check the state of the machine using the driver, for a more
					// accurate read
					state, err := driver.State(ctx, mid)
					if err != nil {
						plog.Errorf("could not look up machine state: %v", err)
					}

					switch state {
					case machine.MachineStateExited, machine.MachineStateDead:
						if mcfg.DestroyOnExit {
							plog.Infof("removing %s...", mid.ShortString())
							if err := driver.Destroy(ctx, mid); err != nil {
								plog.Errorf("could not remove machine: %v: ", err)
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
						plog.Infof("%s : %s", mid.ShortString(), state.String())
						switch state {
						case machine.MachineStateExited, machine.MachineStateDead:
							if mcfg.DestroyOnExit {
								plog.Infof("removing %s...", mid.ShortString())
								if err := driver.Destroy(ctx, mid); err != nil {
									plog.Errorf("could not remove machine: %v: ", err)
								}
							}
							observations.Done(mid)
							return
						}

					case err := <-errs:
						if !errors.Is(err, qmp.ErrAcceptedNonEvent) {
							plog.Errorf("%v", err)
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

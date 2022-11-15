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

package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine"
	machinedriver "kraftkit.sh/machine/driver"
	machinedriveropts "kraftkit.sh/machine/driveropts"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/utils"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/internal/logger"

	"github.com/MakeNowJust/heredoc"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/moby/moby/pkg/namesgenerator"
	"github.com/spf13/cobra"
)

type runOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams

	// Command-line arguments
	Architecture  string
	Detach        bool
	DisableAccel  bool
	Hypervisor    string
	Memory        int
	NoMonitor     bool
	PinCPUs       string
	Platform      string
	Remove        bool
	Target        string
	Volumes       []string
	WithKernelDbg bool
}

func RunCmd(f *cmdfactory.Factory) *cobra.Command {
	cmd, err := cmdutil.NewCmd(f, "run",
		cmdutil.WithSubcmds(),
	)
	if err != nil {
		panic("could not initialize 'kraft run' command")
	}

	opts := &runOptions{
		PackageManager: f.PackageManager,
		ConfigManager:  f.ConfigManager,
		Logger:         f.Logger,
		IO:             f.IOStreams,
	}

	cmd.Short = "Run a unikernel"
	cmd.Use = "run [FLAGS] [PROJECT|KERNEL] [ARGS]"
	cmd.Aliases = []string{"launch", "r"}
	cmd.Long = heredoc.Doc(`
		Launch a unikernel`)
	cmd.Example = heredoc.Doc(`
		# Run a unikernel kernel image
		kraft run path/to/kernel-x86_64-kvm

		# Run a project which only has one target
		kraft run path/to/project
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		opts.Hypervisor = cmd.Flag("hypervisor").Value.String()

		return runRun(opts, args...)
	}

	cmd.Flags().BoolVarP(
		&opts.Detach,
		"detach", "d",
		false,
		"Run unikernel in background.",
	)

	cmd.Flags().BoolVar(
		&opts.WithKernelDbg,
		"symbolic",
		false,
		"Use the debuggable (symbolic) unikernel.",
	)

	cmd.Flags().BoolVarP(
		&opts.DisableAccel,
		"disable-acceleration", "W",
		false,
		"Disable acceleration of CPU (usually enables TCG).",
	)

	cmd.Flags().IntVarP(
		&opts.Memory,
		"memory", "M",
		64,
		"Assign MB memory to the unikernel.",
	)

	cmd.Flags().StringVarP(
		&opts.Target,
		"target", "t",
		"",
		"Explicitly use the defined project target.",
	)

	cmd.Flags().VarP(
		cmdutil.NewEnumFlag(machinedriver.DriverNames(), "auto"),
		"hypervisor",
		"H",
		"Set the hypervisor machine driver.",
	)

	cmd.Flags().StringVar(
		&opts.Architecture,
		"arch",
		"",
		"Filter the creation of the package by architecture of known targets",
	)

	cmd.Flags().StringVar(
		&opts.Platform,
		"plat",
		"",
		"Filter the creation of the package by platform of known targets",
	)

	cmd.Flags().BoolVar(
		&opts.NoMonitor,
		"no-monitor",
		false,
		"Do not spawn a (or attach to an existing) KraftKit unikernel monitor",
	)

	cmd.Flags().BoolVar(
		&opts.Remove,
		"rm",
		false,
		"Automatically remove the unikernel when it shutsdown",
	)

	return cmd
}

func runRun(opts *runOptions, args ...string) error {
	var err error

	plog, err := opts.Logger()
	if err != nil {
		return err
	}

	cfgm, err := opts.ConfigManager()
	if err != nil {
		return err
	}

	var driverType *machinedriver.DriverType
	if opts.Hypervisor == "auto" {
		dt, err := machinedriver.DetectHostHypervisor()
		if err != nil {
			return err
		}
		driverType = &dt
	} else if opts.Hypervisor == "config" {
		opts.Hypervisor = cfgm.Config.DefaultPlat
	}

	if driverType == nil && len(opts.Hypervisor) > 0 && !utils.Contains(machinedriver.DriverNames(), opts.Hypervisor) {
		return fmt.Errorf("unknown hypervisor driver: %s", opts.Hypervisor)
	}

	debug := logger.LogLevelFromString(cfgm.Config.Log.Level) >= logger.DEBUG
	var msopts []machine.MachineStoreOption
	if debug {
		msopts = append(msopts,
			machine.WithMachineStoreLogger(plog),
		)
	}

	store, err := machine.NewMachineStoreFromPath(cfgm.Config.RuntimeDir, msopts...)
	if err != nil {
		return fmt.Errorf("could not access machine store: %v", err)
	}

	driver, err := machinedriver.New(*driverType,
		machinedriveropts.WithBackground(opts.Detach),
		machinedriveropts.WithRuntimeDir(cfgm.Config.RuntimeDir),
		machinedriveropts.WithMachineStore(store),
		machinedriveropts.WithLogger(plog),
		machinedriveropts.WithDebug(debug),
		machinedriveropts.WithExecOptions(
			exec.WithStdout(os.Stdout),
			exec.WithStderr(os.Stderr),
		),
	)
	if err != nil {
		return err
	}

	mopts := []machine.MachineOption{
		machine.WithDriverName(driverType.String()),
		machine.WithDestroyOnExit(opts.Remove),
	}

	// The following sequence checks the position argument of `kraft run ENTITY`
	// where ENTITY can either be:
	// a). path to a project which either uses the only specified target or one
	//     specified via the -t flag;
	// b). a target defined within the context of `workdir` (which is either set
	//     via -w or is the current working directory); or
	// c). path to a kernel.
	var workdir string
	var entity string
	var kernelArgs []string

	// Determine if more than one positional arguments have been provided.  If
	// this is the case, everything after the first position argument are kernel
	// parameters which should be passed appropriately.
	if len(args) > 1 {
		entity = args[0]
		kernelArgs = args[1:]
	} else if len(args) == 1 {
		entity = args[0]
	}

	// a). path to a project
	if len(entity) > 0 && app.IsWorkdirInitialized(entity) {
		workdir = entity

		// Otherwise use the current working directory
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		if app.IsWorkdirInitialized(cwd) {
			workdir = cwd
			kernelArgs = args
		}
	}

	// b). use a defined working directory as a Unikraft project
	if len(workdir) > 0 {
		target := opts.Target
		projectOpts, err := app.NewProjectOptions(
			nil,
			app.WithLogger(plog),
			app.WithWorkingDirectory(workdir),
			app.WithDefaultConfigPath(),
			// app.WithPackageManager(&pm),
			app.WithResolvedPaths(true),
			app.WithDotConfig(false),
		)
		if err != nil {
			return err
		}

		// Interpret the application
		app, err := app.NewApplicationFromOptions(projectOpts)
		if err != nil {
			return err
		}

		if len(app.TargetNames()) == 1 {
			target = app.TargetNames()[0]
			if len(opts.Target) > 0 && opts.Target != target {
				return fmt.Errorf("unknown target: %s", opts.Target)
			}

		} else if len(target) == 0 && len(app.TargetNames()) > 1 {
			if cfgm.Config.NoPrompt {
				return fmt.Errorf("with 'no prompt' enabled please select a target")
			}

			sp := selection.New("select target:",
				selection.Choices(app.TargetNames()),
			)
			sp.Filter = nil

			selectedTarget, err := sp.RunPrompt()
			if err != nil {
				return err
			}

			target = selectedTarget.String

		} else if target != "" && utils.Contains(app.TargetNames(), target) {
			return fmt.Errorf("unknown target: %s", target)
		}

		t, err := app.TargetByName(target)
		if err != nil {
			return err
		}

		// Validate selection of target
		if len(opts.Architecture) > 0 && (opts.Architecture != t.Architecture.Name()) {
			return fmt.Errorf("selected target (%s) does not match specified architecture (%s)", t.ArchPlatString(), opts.Architecture)
		}
		if len(opts.Platform) > 0 && (opts.Platform != t.Platform.Name()) {
			return fmt.Errorf("selected target (%s) does not match specified platform (%s)", t.ArchPlatString(), opts.Platform)
		}

		mopts = append(mopts,
			machine.WithArchitecture(t.Architecture.Name()),
			machine.WithPlatform(t.Platform.Name()),
			machine.WithName(machine.MachineName(t.Name())),
			machine.WithAcceleration(!opts.DisableAccel),
			machine.WithSource("project://"+app.Name()+":"+t.Name()),
		)

		// Use the symbolic debuggable kernel image?
		if opts.WithKernelDbg {
			mopts = append(mopts, machine.WithKernel(t.KernelDbg))
		} else {
			mopts = append(mopts, machine.WithKernel(t.Kernel))
		}

		// If no entity was set earlier and we're not within the context of a working
		// directory, then we're unsure what to run
	} else if len(entity) == 0 {
		return fmt.Errorf("cannot run without providing a working directory (and target), kernel binary or package")

		// c). Is the provided first position argument a binary image?
	} else if f, err := os.Stat(entity); err == nil && !f.IsDir() {
		if len(opts.Architecture) == 0 || len(opts.Platform) == 0 {
			return fmt.Errorf("cannot use `kraft run KERNEL` without specifying --arch and --plat")
		}

		mopts = append(mopts,
			machine.WithArchitecture(opts.Architecture),
			machine.WithPlatform(opts.Platform),
			machine.WithName(machine.MachineName(namesgenerator.GetRandomName(0))),
			machine.WithKernel(entity),
			machine.WithSource("kernel://"+filepath.Base(entity)),
		)
	} else {
		return fmt.Errorf("could not determine what to run: %s", entity)
	}

	mopts = append(mopts,
		machine.WithMemorySize(uint64(opts.Memory)),
		machine.WithArguments(kernelArgs),
	)

	ctx := context.Background()

	// Create the machine
	mid, err := driver.Create(ctx, mopts...)
	if err != nil {
		return err
	}

	plog.Infof("created %s instance %s", driverType.String(), mid.ShortString())

	// Start the machine
	if err := driver.Start(ctx, mid); err != nil {
		return err
	}

	if !opts.NoMonitor {
		// Spawn an event monitor or attach to an existing monitor
		_, err = os.Stat(cfgm.Config.EventsPidFile)
		if err != nil && os.IsNotExist(err) {
			plog.Debugf("launching event monitor...")

			// Spawn and detach a new events monitor
			e, err := exec.NewExecutable(os.Args[0], nil, "events", "--quit-together")
			if err != nil {
				return err
			}

			process, err := exec.NewProcessFromExecutable(e,
				exec.WithDetach(true),
			)
			if err != nil {
				return err
			}

			if err := process.Start(); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		// TODO: Failsafe check if a a pidfile for the events monitor exists, let's
		// check that it is active
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctrlc := make(chan os.Signal, 1)
	signal.Notify(ctrlc, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ctrlc // wait for Ctrl+C
		fmt.Printf("\n")

		// Remove the instance on Ctrl+C if the --rm flag is passed
		if opts.Remove {
			plog.Infof("removing %s...", mid.ShortString())
			if err := driver.Destroy(ctx, mid); err != nil {
				plog.Errorf("could not remove %s: %v", mid, err)
			}
		}

		cancel()
	}()

	// Tail the logs if -d|--detach is not provided
	if !opts.Detach {
		plog.Infof("starting to tail %s logs...", mid.ShortString())

		go func() {
			events, errs, err := driver.ListenStatusUpdate(ctx, mid)
			if err != nil {
				plog.Errorf("could not listen for machine updates: %v", err)
				ctrlc <- syscall.SIGTERM
				if !opts.Remove {
					cancel()
				}
			}

			for {
				// Wait on either channel
				select {
				case status := <-events:
					switch status {
					case machine.MachineStateExited, machine.MachineStateDead:
						ctrlc <- syscall.SIGTERM
						if !opts.Remove {
							cancel()
						}
						return
					}

				case err := <-errs:
					plog.Errorf("received event error: %v", err)
					return
				}
			}
		}()

		driver.TailWriter(ctx, mid, opts.IO.Out)
	}

	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/MakeNowJust/heredoc"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/moby/moby/pkg/namesgenerator"
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
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
	"kraftkit.sh/utils"
)

type Run struct {
	Architecture  string `long:"arch" short:"m" usage:"Set the architecture"`
	Detach        bool   `long:"detach" short:"d" usage:"Run unikernel in background"`
	DisableAccel  bool   `long:"disable-acceleration" short:"W" usage:"Disable acceleration of CPU (usually enables TCG)"`
	Hypervisor    string
	Memory        int    `long:"memory" short:"M" usage:"Assign MB memory to the unikernel"`
	Name          string `long:"name" short:"n" usage:"Name of the instance"`
	NoMonitor     bool   `long:"no-monitor" usage:"Do not spawn a (or attach to an existing) an instance monitor"`
	Platform      string `long:"plat" short:"p" usage:"Set the platform"`
	Remove        bool   `long:"rm" usage:"Automatically remove the unikernel when it shutsdown"`
	Target        string `long:"target" short:"t" usage:"Explicitly use the defined project target"`
	WithKernelDbg bool   `long:"symbolic" usage:"Use the debuggable (symbolic) unikernel"`
}

func New() *cobra.Command {
	cmd := cmdfactory.New(&Run{}, cobra.Command{
		Short:   "Run a unikernel",
		Use:     "run [FLAGS] PROJECT|KERNEL -- [UNIKRAFT ARGS] -- [APP ARGS]",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"launch", "r"},
		Long: heredoc.Doc(`
			Launch a unikernel`),
		Example: heredoc.Doc(`
			# Run a unikernel kernel image
			kraft run path/to/kernel-x86_64-kvm

			# Run a project which only has one target
			kraft run path/to/project`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "run",
		},
	})

	cmd.Flags().VarP(
		cmdfactory.NewEnumFlag(machinedriver.DriverNames(), "auto"),
		"hypervisor",
		"H",
		"Set the hypervisor machine driver.",
	)

	return cmd
}

func (opts *Run) Pre(cmd *cobra.Command, args []string) error {
	opts.Hypervisor = cmd.Flag("hypervisor").Value.String()
	return nil
}

func (opts *Run) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	driverType := machinedriver.UnknownDriver

	if opts.Hypervisor == "auto" {
		driverType, err = machinedriver.DetectHostHypervisor()
		if err != nil {
			return err
		}
	} else if opts.Hypervisor == "config" {
		opts.Hypervisor = config.G[config.KraftKit](ctx).DefaultPlat
	} else {
		driverType = machinedriver.DriverTypeFromName(opts.Hypervisor)
	}

	if driverType == machinedriver.UnknownDriver {
		return fmt.Errorf("unknown hypervisor driver: %s", opts.Hypervisor)
	}

	debug := log.Levels()[config.G[config.KraftKit](ctx).Log.Level] >= logrus.DebugLevel
	store, err := machine.NewMachineStoreFromPath(config.G[config.KraftKit](ctx).RuntimeDir)
	if err != nil {
		return fmt.Errorf("could not access machine store: %v", err)
	}

	driver, err := machinedriver.New(driverType,
		machinedriveropts.WithBackground(opts.Detach),
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

	mopts := []machine.MachineOption{
		machine.WithDriverName(driverType.String()),
		machine.WithDestroyOnExit(opts.Remove),
	}

	// The following sequence checks the position argument of `kraft run ENTITY`
	// where ENTITY can either be:
	// a). path to a project which either uses the only specified target or one
	//     specified via the -t flag, e.g.:
	//
	//     $ kraft run path/to/project # with 1 default target
	//     # or for multiple targets
	//     $ kraft run -t target-name path/to/project
	//
	// b). path to a kernel, e.g.:
	//
	//     $ kraft run path/to/kernel
	//
	// c). a target defined within the context of `workdir` (which is either set
	//     via -w or is the current working directory), e.g.:
	//
	//     $ cd path/to/project
	//     $ kraft run target-name
	//     # or
	//     $ kraft run -w path/to/project target-name
	//
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

	// b). Is the provided first position argument a binary image?
	if f, err := os.Stat(entity); err == nil && !f.IsDir() {
		if len(opts.Architecture) == 0 || len(opts.Platform) == 0 {
			return fmt.Errorf("cannot use `kraft run KERNEL` without specifying --arch and --plat")
		}

		if opts.Name == "" {
			opts.Name = namesgenerator.GetRandomName(0)
		}

		mopts = append(mopts,
			machine.WithArchitecture(opts.Architecture),
			machine.WithPlatform(opts.Platform),
			machine.WithName(machine.MachineName(opts.Name)),
			machine.WithKernel(entity),
			machine.WithSource("kernel://"+filepath.Base(entity)),
		)

		// c). use a defined working directory as a Unikraft project
	} else if len(workdir) > 0 {
		target := opts.Target
		project, err := app.NewProjectFromOptions(
			ctx,
			app.WithProjectWorkdir(workdir),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		if len(project.TargetNames()) == 1 {
			target = project.TargetNames()[0]
			if len(opts.Target) > 0 && opts.Target != target {
				return fmt.Errorf("unknown target: %s", opts.Target)
			}

		} else if target == "" && len(project.Targets()) > 1 {
			if config.G[config.KraftKit](ctx).NoPrompt {
				return fmt.Errorf("with 'no prompt' enabled please select a target")
			}

			sp := selection.New("select target:", selectionTargets(project.Targets()))
			sp.Filter = nil

			selectedTarget, err := sp.RunPrompt()
			if err != nil {
				return err
			}

			target = selectedTarget

		} else if target != "" && utils.Contains(project.TargetNames(), target) {
			return fmt.Errorf("unknown target: %s", target)
		}

		t, err := project.TargetByName(target)
		if err != nil {
			return err
		}

		// Validate selection of target
		if len(opts.Architecture) > 0 && (opts.Architecture != t.Architecture().Name()) {
			return fmt.Errorf("selected target (%s) does not match specified architecture (%s)", t.ArchPlatString(), opts.Architecture)
		}
		if len(opts.Platform) > 0 && (opts.Platform != t.Platform().Name()) {
			return fmt.Errorf("selected target (%s) does not match specified platform (%s)", t.ArchPlatString(), opts.Platform)
		}

		name := t.Name()
		if opts.Name != "" {
			name = opts.Name
		}

		mopts = append(mopts,
			machine.WithArchitecture(t.Architecture().Name()),
			machine.WithPlatform(t.Platform().Name()),
			machine.WithName(machine.MachineName(name)),
			machine.WithAcceleration(!opts.DisableAccel),
			machine.WithSource("project://"+project.Name()+":"+t.Name()),
		)

		// Use the symbolic debuggable kernel image?
		if opts.WithKernelDbg {
			mopts = append(mopts, machine.WithKernel(t.KernelDbg()))
		} else {
			mopts = append(mopts, machine.WithKernel(t.Kernel()))
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

		if opts.Name == "" {
			opts.Name = namesgenerator.GetRandomName(0)
		}

		mopts = append(mopts,
			machine.WithArchitecture(opts.Architecture),
			machine.WithPlatform(opts.Platform),
			machine.WithName(machine.MachineName(opts.Name)),
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

	// Create the machine
	mid, err := driver.Create(ctx, mopts...)
	if err != nil {
		return err
	}

	log.G(ctx).Infof("created %s instance %s", driverType.String(), mid.ShortString())

	// Start the machine
	if err := driver.Start(ctx, mid); err != nil {
		return err
	}

	if !opts.NoMonitor {
		// Spawn an event monitor or attach to an existing monitor
		_, err = os.Stat(config.G[config.KraftKit](ctx).EventsPidFile)
		if err != nil && os.IsNotExist(err) {
			log.G(ctx).Debugf("launching event monitor...")

			// Spawn and detach a new events monitor
			e, err := exec.NewExecutable(os.Args[0], nil, "events", "--quit-together")
			if err != nil {
				return err
			}

			process, err := exec.NewProcessFromExecutable(
				e,
				exec.WithDetach(true),
			)
			if err != nil {
				return err
			}

			if err := process.Start(ctx); err != nil {
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
			log.G(ctx).Infof("removing %s...", mid.ShortString())
			if err := driver.Destroy(ctx, mid); err != nil {
				log.G(ctx).Errorf("could not remove %s: %v", mid, err)
			}
		}

		cancel()
	}()

	// Tail the logs if -d|--detach is not provided
	if !opts.Detach {
		log.G(ctx).Infof("starting to tail %s logs...", mid.ShortString())

		go func() {
			events, errs, err := driver.ListenStatusUpdate(ctx, mid)
			if err != nil {
				log.G(ctx).Errorf("could not listen for machine updates: %v", err)
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
					log.G(ctx).Errorf("received event error: %v", err)
					return
				}
			}
		}()

		driver.TailWriter(ctx, mid, iostreams.G(ctx).Out)
	}

	return nil
}

// selectionTargets returns the given target.Targets in a format suitable for
// interactive prompts.
func selectionTargets(tgts target.Targets) []string {
	tgtStrings := make([]string, 0, len(tgts))
	for _, tgt := range tgts {
		tgtStrings = append(tgtStrings, fmt.Sprintf("%s (%s)", tgt.Name(), tgt.ArchPlatString()))
	}

	sort.Strings(tgtStrings)

	return tgtStrings
}

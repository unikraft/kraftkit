// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/MakeNowJust/heredoc"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/moby/moby/pkg/namesgenerator"
	"github.com/rancher/wrangler/pkg/signals"
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
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
	"kraftkit.sh/utils"
)

type Run struct {
	Architecture  string `long:"arch" short:"m" usage:"Set the architecture"`
	Detach        bool   `long:"detach" short:"d" usage:"Run unikernel in background"`
	DisableAccel  bool   `long:"disable-acceleration" short:"W" usage:"Disable acceleration of CPU (usually enables TCG)"`
	Hypervisor    string
	InitRd        string `long:"initrd" short:"i" usage:"Use the specified initrd"`
	Memory        int    `long:"memory" short:"M" usage:"Assign MB memory to the unikernel"`
	Name          string `long:"name" short:"n" usage:"Name of the instance"`
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
		Aliases: []string{"r"},
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

func (opts *Run) Pre(cmd *cobra.Command, _ []string) error {
	opts.Hypervisor = cmd.Flag("hypervisor").Value.String()

	ctx := cmd.Context()
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

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
	// c). A package manager image reference containing a unikernel;
	//
	// d). a target defined within the context of `workdir` (which is either set
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
			machine.WithInitRd(opts.InitRd),
		)

		// c). use the provided package manager
	} else if pm, compatible, err := packmanager.G(ctx).IsCompatible(ctx, entity); err == nil && compatible {
		// First try the local cache of the catalog
		packs, err := pm.Catalog(ctx, packmanager.CatalogQuery{
			Types:   []unikraft.ComponentType{unikraft.ComponentTypeApp},
			Name:    entity,
			NoCache: false,
		})
		if err != nil {
			return err
		}

		if len(packs) > 1 {
			return fmt.Errorf("could not determine what to run, too many options")
		} else if len(packs) == 0 {
			// Second, try accessing the remote catalog
			packs, err = pm.Catalog(ctx, packmanager.CatalogQuery{
				Types:   []unikraft.ComponentType{unikraft.ComponentTypeApp},
				Name:    entity,
				NoCache: true,
			})
			if err != nil {
				return err
			}

			if len(packs) > 1 {
				return fmt.Errorf("could not determine what to run, too many options")
			} else if len(packs) == 0 {
				return fmt.Errorf("not found: %s", entity)
			}
		}

		// Create a temporary directory where the image can be stored
		dir, err := os.MkdirTemp("", "")
		if err != nil {
			return err
		}

		// TODO(nderjung): Somehow this needs to be garbage collected if in
		// detach-mode.
		if !opts.Detach {
			defer os.RemoveAll(dir)
		}

		paramodel, err := paraprogress.NewParaProgress(
			ctx,
			[]*paraprogress.Process{paraprogress.NewProcess(
				fmt.Sprintf("pulling %s", entity),
				func(ctx context.Context, w func(progress float64)) error {
					return packs[0].Pull(
						ctx,
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(dir),
					)
				},
			)},
			paraprogress.IsParallel(false),
			paraprogress.WithRenderer(
				log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
			),
			paraprogress.WithFailFast(true),
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return err
		}

		// Crucially, the catalog should return an interface that also implements
		// target.Target.  This demonstrates that the implementing package can
		// resolve application kernels.
		targ, ok := packs[0].(target.Target)
		if !ok {
			return fmt.Errorf("package does not convert to target")
		}

		if opts.Name == "" {
			opts.Name = namesgenerator.GetRandomName(0)
		}

		mopts = append(mopts,
			machine.WithArchitecture(targ.Architecture().Name()),
			machine.WithPlatform(targ.Platform().Name()),
			machine.WithName(machine.MachineName(opts.Name)),
			machine.WithKernel(targ.Kernel()),
			machine.WithSource(fmt.Sprintf("%s://%s", pm.Format(), entity)),
		)
		// d). use a defined working directory as a Unikraft project
	} else if len(workdir) > 0 {
		targetName := opts.Target
		project, err := app.NewProjectFromOptions(
			ctx,
			app.WithProjectWorkdir(workdir),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		if len(project.TargetNames()) == 1 {
			targetName = project.TargetNames()[0]
			if len(opts.Target) > 0 && opts.Target != targetName {
				return fmt.Errorf("unknown target: %s", opts.Target)
			}

		} else if targetName == "" && len(project.Targets()) > 1 {
			if config.G[config.KraftKit](ctx).NoPrompt {
				return fmt.Errorf("with 'no prompt' enabled please select a target")
			}

			sp := selection.New("select target:", selectionTargets(project.Targets()))
			sp.Filter = nil

			selectedTarget, err := sp.RunPrompt()
			if err != nil {
				return err
			}

			targetName = selectedTarget

		} else if targetName != "" && utils.Contains(project.TargetNames(), targetName) {
			return fmt.Errorf("unknown target: %s", targetName)
		}

		t, err := project.TargetByName(targetName)
		if err != nil {
			return err
		}

		// Validate selection of target
		if len(opts.Architecture) > 0 && (opts.Architecture != t.Architecture().Name()) {
			return fmt.Errorf("selected target (%s) does not match specified architecture (%s)", target.TargetPlatArchName(t), opts.Architecture)
		}
		if len(opts.Platform) > 0 && (opts.Platform != t.Platform().Name()) {
			return fmt.Errorf("selected target (%s) does not match specified platform (%s)", target.TargetPlatArchName(t), opts.Platform)
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
			machine.WithInitRd(opts.InitRd),
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
			machine.WithInitRd(opts.InitRd),
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

	log.G(ctx).Debugf("created %s instance %s", driverType.String(), mid.ShortString())

	// Tail the logs if -d|--detach is not provided
	if !opts.Detach {
		go func() {
			events, errs, err := driver.ListenStatusUpdate(ctx, mid)
			if err != nil {
				log.G(ctx).Errorf("could not listen for machine updates: %v", err)
				signals.RequestShutdown()
				return
			}

			log.G(ctx).Debug("waiting for machine events")

		loop:
			for {
				// Wait on either channel
				select {
				case status := <-events:
					store.SaveMachineState(mid, status)

					switch status {
					case machine.MachineStateExited, machine.MachineStateDead:
						signals.RequestShutdown()
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
	}

	// Start the machine
	if err := driver.Start(ctx, mid); err != nil {
		signals.RequestShutdown()
		return err
	}

	if !opts.Detach {
		driver.TailWriter(ctx, mid, iostreams.G(ctx).Out)

		// Wait for the context to be cancelled, which can occur if a fatal error
		// occurs or the user has requested a SIGINT (Ctrl+C).
		<-ctx.Done()

		// Reading the state to determine whether to invoke a stop request but this
		// also simultaneously updates the state if it has exited.
		state, stateErr := driver.State(ctx, mid)

		// Remove the instance on Ctrl+C if the --rm flag is passed
		if opts.Remove {
			log.G(ctx).Debugf("removing %s", mid.ShortString())
			if err := driver.Destroy(ctx, mid); stateErr == nil && err != nil {
				log.G(ctx).
					WithField("mid", mid).
					Errorf("could not remove: %v", err)
			}
		} else if state == machine.MachineStateRunning {
			log.G(ctx).Debugf("stopping %s", mid.ShortString())
			if err := driver.Stop(ctx, mid); stateErr == nil && err != nil {
				log.G(ctx).
					WithField("mid", mid).
					Errorf("could not stop: %v", err)
			}
		}
	}

	return nil
}

// selectionTargets returns the given target.Targets in a format suitable for
// interactive prompts.
func selectionTargets(tgts target.Targets) []string {
	tgtStrings := make([]string, 0, len(tgts))
	for _, tgt := range tgts {
		tgtStrings = append(tgtStrings, fmt.Sprintf("%s (%s)", tgt.Name(), target.TargetPlatArchName(tgt)))
	}

	sort.Strings(tgtStrings)

	return tgtStrings
}

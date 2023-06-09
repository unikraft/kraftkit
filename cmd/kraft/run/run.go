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

	"github.com/MakeNowJust/heredoc"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	machinename "kraftkit.sh/machine/name"
	mplatform "kraftkit.sh/machine/platform"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

type Run struct {
	Architecture  string `long:"arch" short:"m" usage:"Set the architecture"`
	Detach        bool   `long:"detach" short:"d" usage:"Run unikernel in background"`
	DisableAccel  bool   `long:"disable-acceleration" short:"W" usage:"Disable acceleration of CPU (usually enables TCG)"`
	InitRd        string `long:"initrd" short:"i" usage:"Use the specified initrd"`
	Memory        string `long:"memory" short:"M" usage:"Assign MB memory to the unikernel" default:"64M"`
	Name          string `long:"name" short:"n" usage:"Name of the instance"`
	platform      string
	Remove        bool     `long:"rm" usage:"Automatically remove the unikernel when it shutsdown"`
	Target        string   `long:"target" short:"t" usage:"Explicitly use the defined project target"`
	WithKernelDbg bool     `long:"symbolic" usage:"Use the debuggable (symbolic) unikernel"`
	KernelArgs    []string `long:"kernel-arg" short:"a" usage:"Set additional kernel arguments"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Run{}, cobra.Command{
		Short:   "Run a unikernel",
		Use:     "run [FLAGS] PROJECT|KERNEL -- [APP ARGS]",
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
	if err != nil {
		panic(err)
	}

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag(set.NewStringSet(mplatform.DriverNames()...).Add("auto").ToSlice(), "auto"),
		"plat",
		"Set the platform virtual machine monitor driver.",
	)

	return cmd
}

func (opts *Run) Pre(cmd *cobra.Command, _ []string) error {
	opts.platform = cmd.Flag("plat").Value.String()

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
	platform := mplatform.PlatformUnknown
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

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

	machine := &machineapi.Machine{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: machineapi.MachineSpec{
			Rootfs: opts.InitRd,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{},
			},
		},
	}

	controller, err := strategy.NewMachineV1alpha1(ctx)
	if err != nil {
		return err
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
	var appArgs []string

	// Determine if more than one positional arguments have been provided.  If
	// this is the case, everything after the first position argument are kernel
	// parameters which should be passed appropriately.
	if len(args) > 1 {
		entity = args[0]
		appArgs = args[1:]
	} else if len(args) == 1 {
		entity = args[0]
	}

	// a). path to a project
	if len(entity) > 0 && app.IsWorkdirInitialized(entity) {
		workdir = entity

		// Otherwise use the current working directory
	} else {
		if app.IsWorkdirInitialized(cwd) {
			workdir = cwd
			appArgs = args
		}
	}

	// b). Is the provided first position argument a binary image?
	if f, err := os.Stat(entity); err == nil && !f.IsDir() {
		if len(opts.Architecture) == 0 || len(opts.platform) == 0 {
			return fmt.Errorf("cannot use `kraft run KERNEL` without specifying --arch and --plat")
		}

		appArgs = args[1:]
		entity = filepath.Join(cwd, entity)
		machine.Spec.Architecture = opts.Architecture
		machine.Spec.Platform = opts.platform
		machine.Spec.Kernel = "kernel://" + filepath.Base(entity)
		machine.Status.KernelPath = entity

		// c). use the provided package manager
	} else if pm, compatible, err := packmanager.G(ctx).IsCompatible(ctx, entity); err == nil && compatible {
		// First try the local cache of the catalog
		packs, err := pm.Catalog(ctx,
			packmanager.WithTypes(unikraft.ComponentTypeApp),
			packmanager.WithName(entity),
			packmanager.WithCache(true),
		)
		if err != nil {
			return err
		}

		if len(packs) > 1 {
			return fmt.Errorf("could not determine what to run, too many options")
		} else if len(packs) == 0 {
			// Second, try accessing the remote catalog
			packs, err = pm.Catalog(ctx,
				packmanager.WithTypes(unikraft.ComponentTypeApp),
				packmanager.WithName(entity),
				packmanager.WithCache(false),
			)
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

		appArgs = args[1:]
		machine.Spec.Architecture = targ.Architecture().Name()
		machine.Spec.Platform = targ.Platform().Name()
		machine.Spec.Kernel = fmt.Sprintf("%s://%s", pm.Format(), entity)

		// Use the symbolic debuggable kernel image?
		if opts.WithKernelDbg {
			machine.Status.KernelPath = targ.KernelDbg()
		} else {
			machine.Status.KernelPath = targ.Kernel()
		}

		// d). use a defined working directory as a Unikraft project
	} else if len(workdir) > 0 {
		project, err := app.NewProjectFromOptions(
			ctx,
			app.WithProjectWorkdir(workdir),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		// Filter project targets by any provided CLI options
		targets := cli.FilterTargets(
			project.Targets(),
			opts.Architecture,
			opts.platform,
			opts.Target,
		)

		var t target.Target

		switch {
		case len(targets) == 1:
			t = targets[0]

		case config.G[config.KraftKit](ctx).NoPrompt && len(targets) > 1:
			return fmt.Errorf("could not determine what to run based on provided CLI arguments")

		case config.G[config.KraftKit](ctx).NoPrompt && len(targets) == 0:
			return fmt.Errorf("could not match any target")

		default:
			t, err = cli.SelectTarget(targets)
			if err != nil {
				return err
			}
		}

		machine.Spec.Kernel = "project://" + project.Name() + ":" + t.Name()
		machine.Spec.Architecture = t.Architecture().Name()
		machine.Spec.Platform = t.Platform().Name()

		// Use the symbolic debuggable kernel image?
		if opts.WithKernelDbg {
			machine.Status.KernelPath = t.KernelDbg()
		} else {
			machine.Status.KernelPath = t.Kernel()
		}

		// If no entity was set earlier and we're not within the context of a working
		// directory, then we're unsure what to run
	} else if len(entity) == 0 {
		return fmt.Errorf("cannot run without providing a working directory (and target), kernel binary or package")

		// c). Is the provided first position argument a binary image?
	} else if f, err := os.Stat(entity); err == nil && !f.IsDir() {
		if len(opts.Architecture) == 0 || len(opts.platform) == 0 {
			return fmt.Errorf("cannot use `kraft run KERNEL` without specifying --arch and --plat")
		}

		machine.Spec.Kernel = "kernel://" + entity
		machine.Spec.Architecture = opts.Architecture
		machine.Spec.Platform = opts.platform
		machine.Status.KernelPath = entity
	} else {
		return fmt.Errorf("could not determine what to run: %s", entity)
	}

	if opts.Name != "" {
		// Check if this name has been previously used
		machines, err := controller.List(ctx, &machineapi.MachineList{})
		if err != nil {
			return err
		}

		for _, machine := range machines.Items {
			if opts.Name == machine.Name {
				return fmt.Errorf("machine instance name already in use: %s", opts.Name)
			}
		}
	} else {
		opts.Name = machinename.NewRandomMachineName(0)
	}

	machine.ObjectMeta.Name = opts.Name
	machine.Spec.KernelArgs = opts.KernelArgs
	machine.Spec.ApplicationArgs = appArgs

	if len(opts.Memory) > 0 {
		quantity, err := resource.ParseQuantity(opts.Memory)
		if err != nil {
			return err
		}

		machine.Spec.Resources.Requests[corev1.ResourceMemory] = quantity
	}

	// Create the machine
	machine, err = controller.Create(ctx, machine)
	if err != nil {
		return err
	}

	// Tail the logs if -d|--detach is not provided
	if !opts.Detach {
		go func() {
			events, errs, err := controller.Watch(ctx, machine)
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
				case update := <-events:
					switch update.Status.State {
					case machineapi.MachineStateExited, machineapi.MachineStateFailed:
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
	machine, err = controller.Start(ctx, machine)
	if err != nil {
		signals.RequestShutdown()
		return err
	}

	if !opts.Detach {
		logs, errs, err := controller.Logs(ctx, machine)
		if err != nil {
			signals.RequestShutdown()
			return fmt.Errorf("could not listen for machine logs: %v", err)
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

		// Remove the instance on Ctrl+C if the --rm flag is passed
		if opts.Remove {
			if _, err := controller.Stop(ctx, machine); err != nil {
				return fmt.Errorf("could not stop: %v", err)
			}
			if _, err := controller.Delete(ctx, machine); err != nil {
				return fmt.Errorf("could not remove: %v", err)
			}
		}
	} else {
		// Output the name of the instance such that it can be piped
		fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", machine.Name)
	}

	return nil
}

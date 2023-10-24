// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"context"
	"fmt"
	"os"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/unikraft/app"
	ukarch "kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/target"
)

type runnerKraftfileRuntime struct {
	args    []string
	project app.Application
}

// String implements Runner.
func (runner *runnerKraftfileRuntime) String() string {
	return "kraftfile-runtime"
}

// Runnable implements Runner.
func (runner *runnerKraftfileRuntime) Runnable(ctx context.Context, opts *RunOptions, args ...string) (bool, error) {
	var err error

	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("getting current working directory: %w", err)
	}

	if len(args) == 0 {
		opts.workdir = cwd
	} else {
		opts.workdir = cwd
		runner.args = args
		if f, err := os.Stat(args[0]); err == nil && f.IsDir() {
			opts.workdir = args[0]
			runner.args = args[1:]
		}
	}

	if !app.IsWorkdirInitialized(opts.workdir) {
		return false, fmt.Errorf("path is not project: %s", opts.workdir)
	}

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	runner.project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return false, fmt.Errorf("could not instantiate project directory %s: %v", opts.workdir, err)
	}

	if runner.project.Runtime() == nil {
		return false, fmt.Errorf("cannot run project without runtime directive")
	}

	return true, nil
}

// Prepare implements Runner.
func (runner *runnerKraftfileRuntime) Prepare(ctx context.Context, opts *RunOptions, machine *machineapi.Machine, args ...string) error {
	var err error
	var targ target.Target

	targets := runner.project.Targets()
	qopts := []packmanager.QueryOption{
		packmanager.WithName(runner.project.Runtime().Name()),
		packmanager.WithVersion(runner.project.Runtime().Version()),
		packmanager.WithUpdate(true),
	}

	if len(targets) == 1 {
		targ = targets[0]
	} else if len(targets) > 1 {
		// Filter project targets by any provided CLI options
		targets = target.Filter(
			targets,
			opts.Architecture,
			opts.platform.String(),
			opts.Target,
		)

		switch {
		case len(targets) == 0:
			return fmt.Errorf("could not detect any project targets based on plat=\"%s\" arch=\"%s\"", opts.platform.String(), opts.Architecture)

		case len(targets) == 1:
			targ = targets[0]

		case config.G[config.KraftKit](ctx).NoPrompt && len(targets) > 1:
			return fmt.Errorf("could not determine what to run based on provided CLI arguments")

		default:
			targ, err = target.Select(targets)
			if err != nil {
				return fmt.Errorf("could not select target: %v", err)
			}
		}
	}

	if targ != nil {
		var kconfigs []string
		for _, kc := range targ.KConfig() {
			kconfigs = append(kconfigs, kc.String())
		}

		qopts = append(qopts,
			packmanager.WithPlatform(targ.Platform().Name()),
			packmanager.WithArchitecture(targ.Architecture().Name()),
			packmanager.WithKConfig(kconfigs),
		)
	} else {
		arch, err := ukarch.HostArchitecture()
		if err != nil {
			return fmt.Errorf("could not get host architecture: %w", err)
		}

		plat, _, err := platform.Detect(ctx)
		if err != nil {
			return fmt.Errorf("could not get host platform: %w", err)
		}

		// Use host information
		qopts = append(qopts,
			packmanager.WithPlatform(plat.String()),
			packmanager.WithArchitecture(arch),
		)
	}

	packs, err := packmanager.G(ctx).Catalog(ctx, qopts...)
	if err != nil {
		return fmt.Errorf("could not query catalog: %w", err)
	} else if len(packs) == 0 {
		return fmt.Errorf("coud not find runtime '%s'", runner.project.Runtime().Name())
	} else if len(packs) > 1 {
		return fmt.Errorf("could not find runtime: too many options")
	}

	if runner.project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = runner.project.Rootfs()
	}

	// Create a temporary directory where the image can be stored
	tempDir, err := os.MkdirTemp("", "kraft-run-")
	if err != nil {
		return err
	}

	paramodel, err := paraprogress.NewParaProgress(
		ctx,
		[]*paraprogress.Process{paraprogress.NewProcess(
			fmt.Sprintf("pulling %s", packs[0].String()),
			func(ctx context.Context, w func(progress float64)) error {
				popts := []pack.PullOption{
					pack.WithPullWorkdir(tempDir),
				}
				if log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) == log.FANCY {
					popts = append(popts, pack.WithPullProgressFunc(w))
				}

				return packs[0].Pull(
					ctx,
					popts...,
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
	runtime, ok := packs[0].(target.Target)
	if !ok {
		return fmt.Errorf("package does not convert to target")
	}

	machine.Spec.Architecture = runtime.Architecture().Name()
	machine.Spec.Platform = runtime.Platform().Name()
	machine.Spec.Kernel = fmt.Sprintf("%s://%s:%s", packs[0].Format(), runtime.Name(), runtime.Version())

	if len(runner.project.Command()) > 0 {
		machine.Spec.ApplicationArgs = runner.project.Command()
	} else if len(runtime.Command()) > 0 {
		machine.Spec.ApplicationArgs = runtime.Command()
	}

	// Use the symbolic debuggable kernel image?
	if opts.WithKernelDbg {
		machine.Status.KernelPath = runtime.KernelDbg()
	} else {
		machine.Status.KernelPath = runtime.Kernel()
	}

	if opts.Rootfs == "" {
		if runner.project.Rootfs() != "" {
			opts.Rootfs = runner.project.Rootfs()
		} else if runtime.Initrd() != nil {
			machine.Status.InitrdPath, err = runtime.Initrd().Build(ctx)
			if err != nil {
				return err
			}
		}
	}

	if err := opts.parseKraftfileVolumes(ctx, runner.project, machine); err != nil {
		return err
	}

	return nil
}

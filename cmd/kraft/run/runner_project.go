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
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

// runnerProject is the runner for a path to a project which either uses the
// only provided target (single) or one specified via the -t|--target flag
// (multiple), e.g.:
//
//	$ kraft run                            // single target in cwd.
//	$ kraft run path/to/project            // single target at path.
//	$ kraft run -t target                  // multiple targets in cwd.
//	$ kraft run -t target path/to/project  // multiple targets at path.
type runnerProject struct {
	workdir string
	args    []string
}

// String implements Runner.
func (runner *runnerProject) String() string {
	return "project"
}

// Runnable implements Runner.
func (runner *runnerProject) Runnable(ctx context.Context, opts *Run, args ...string) (bool, error) {
	var err error

	cwd, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("getting current working directory: %w", err)
	}

	if len(args) == 0 {
		runner.workdir = cwd
	} else {
		runner.workdir = cwd
		runner.args = args
		if f, err := os.Stat(args[0]); err == nil && f.IsDir() {
			runner.workdir = args[0]
			runner.args = args[1:]
		}
	}

	if !app.IsWorkdirInitialized(runner.workdir) {
		return false, fmt.Errorf("path is not project: %s", runner.workdir)
	}

	return true, nil
}

// Prepare implements Runner.
func (runner *runnerProject) Prepare(ctx context.Context, opts *Run, machine *machineapi.Machine, args ...string) error {
	popts := []app.ProjectOption{
		app.WithProjectWorkdir(runner.workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	project, err := app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return fmt.Errorf("could not instantiate project directory %s: %v", runner.workdir, err)
	}

	// Filter project targets by any provided CLI options
	targets := target.Filter(
		project.Targets(),
		opts.Architecture,
		opts.platform.String(),
		opts.Target,
	)

	var t target.Target

	switch {
	case len(targets) == 0:
		return fmt.Errorf("could not detect any project targets based on plat=\"%s\" arch=\"%s\"", opts.platform.String(), opts.Architecture)

	case len(targets) == 1:
		t = targets[0]

	case config.G[config.KraftKit](ctx).NoPrompt && len(targets) > 1:
		return fmt.Errorf("could not determine what to run based on provided CLI arguments")

	default:
		t, err = target.Select(targets)
		if err != nil {
			return fmt.Errorf("could not select target: %v", err)
		}
	}

	// Provide a meaningful name
	targetName := t.Name()
	if targetName == project.Name() || targetName == "" {
		targetName = t.Platform().Name() + "/" + t.Architecture().Name()
	}

	machine.Spec.Kernel = "project://" + project.Name() + ":" + targetName
	machine.Spec.Architecture = t.Architecture().Name()
	machine.Spec.Platform = t.Platform().Name()
	machine.Spec.ApplicationArgs = runner.args

	// Use the symbolic debuggable kernel image?
	if opts.WithKernelDbg {
		machine.Status.KernelPath = t.KernelDbg()
	} else {
		machine.Status.KernelPath = t.Kernel()
	}

	if len(opts.InitRd) > 0 {
		machine.Status.InitrdPath = opts.InitRd
	}

	if _, err := os.Stat(machine.Status.KernelPath); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("cannot run the selected project target '%s' without building the kernel: try running `kraft build` first: %w", targetName, err)
	}

	return nil
}

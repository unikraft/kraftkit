// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	volumeapi "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

// runnerKraftfileUnikraft is the runner for a path to a project which was built
// from source using the provided unikraft source and any auxiliary microlibrary
// components given a provided target (single) or one specified via the
// -t|--target flag (multiple), e.g.:
//
//	$ kraft run                            // single target in cwd.
//	$ kraft run path/to/project            // single target at path.
//	$ kraft run -t target                  // multiple targets in cwd.
//	$ kraft run -t target path/to/project  // multiple targets at path.
type runnerKraftfileUnikraft struct {
	workdir string
	args    []string
	project app.Application
}

// String implements Runner.
func (runner *runnerKraftfileUnikraft) String() string {
	return fmt.Sprintf("run the cwd's Kraftfile and use '%s' as arg(s)", strings.Join(runner.args, " "))
}

// Name implements Runner.
func (runner *runnerKraftfileUnikraft) Name() string {
	return "kraftfile-unikraft"
}

// Runnable implements Runner.
func (runner *runnerKraftfileUnikraft) Runnable(ctx context.Context, opts *RunOptions, args ...string) (bool, error) {
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

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(runner.workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	runner.project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return false, fmt.Errorf("could not instantiate project directory %s: %v", runner.workdir, err)
	}

	if runner.project.Unikraft(ctx) == nil {
		return false, fmt.Errorf("cannot run project build without unikraft")
	}

	return true, nil
}

// Prepare implements Runner.
func (runner *runnerKraftfileUnikraft) Prepare(ctx context.Context, opts *RunOptions, machine *machineapi.Machine, args ...string) error {
	var err error

	// Remove targets which do not have a compiled kernel.
	targets := slices.DeleteFunc(runner.project.Targets(), func(targ target.Target) bool {
		_, err := os.Stat(targ.Kernel())
		return err != nil
	})

	if len(targets) == 0 {
		return fmt.Errorf("cannot run project without any built targets: see `kraft build --help` for more information")
	}

	// Filter project targets by any provided CLI options
	targets = target.Filter(
		targets,
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
	if targetName == runner.project.Name() || targetName == "" {
		targetName = t.Platform().Name() + "/" + t.Architecture().Name()
	}

	machine.Spec.Kernel = "project://" + runner.project.Name() + ":" + targetName
	machine.Spec.Architecture = t.Architecture().Name()
	machine.Spec.Platform = t.Platform().Name()

	if len(runner.args) == 0 {
		runner.args = runner.project.Command()
	}

	noEmbedded := t.KConfig().AllNoOrUnset(
		"CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD",
		"CONFIG_LIBVFSCORE_AUTOMOUNT_CI_EINITRD",
	)

	if runner.project.Rootfs() != "" && opts.Rootfs == "" && noEmbedded {
		opts.Rootfs = runner.project.Rootfs()
	}

	// If automounting is enabled, and an initramfs is provided, set it as a
	// volume if a initram has been provided.
	if t.KConfig().AnyYes(
		"CONFIG_LIBVFSCORE_FSTAB", // Deprecated
		"CONFIG_LIBVFSCORE_AUTOMOUNT_UP",
	) && noEmbedded && (len(machine.Status.InitrdPath) > 0 || len(opts.Rootfs) > 0) {
		machine.Spec.Volumes = append(machine.Spec.Volumes, volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "rootfs",
			},
			Spec: volumeapi.VolumeSpec{
				Driver:      "initrd",
				Destination: "/",
			},
		})
	}

	var kernelArgs []string
	var appArgs []string

	for _, arg := range runner.args {
		if arg == "--" {
			kernelArgs = appArgs
			appArgs = []string{}
			continue
		}
		appArgs = append(appArgs, arg)
	}

	machine.Spec.KernelArgs = kernelArgs
	machine.Spec.ApplicationArgs = appArgs

	// Use the symbolic debuggable kernel image?
	if opts.WithKernelDbg {
		machine.Status.KernelPath = t.KernelDbg()
	} else {
		machine.Status.KernelPath = t.Kernel()
	}

	if _, err := os.Stat(machine.Status.KernelPath); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("cannot run the selected project target '%s' without building the kernel: try running `kraft build` first: %w", targetName, err)
	}

	if err := opts.parseKraftfileVolumes(ctx, runner.project, machine); err != nil {
		return err
	}

	return nil
}

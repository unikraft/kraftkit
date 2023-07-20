// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"k8s.io/apimachinery/pkg/util/uuid"
	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/target"
)

// runnerPackage is a runner for a package defined through a respective
// compatible package manager.  Utilizing the PackageManger interface,
// determination of whether the provided positional argument represents a
// package.  Typically this is used in the OCI usecase where a compatible image
// is referenced which contains a pre-built Unikraft unikernel.  E.g.:
//
//	$ kraft run unikraft.org/helloworld:latest
type runnerPackage struct {
	packName string
	args     []string
	pm       packmanager.PackageManager
}

// String implements Runner.
func (runner *runnerPackage) String() string {
	if runner.pm != nil {
		return runner.pm.Format().String()
	}

	return "package"
}

// Runnable implements Runner.
func (runner *runnerPackage) Runnable(ctx context.Context, opts *Run, args ...string) (bool, error) {
	if len(args) == 0 {
		return false, fmt.Errorf("no arguments supplied")
	}

	runner.packName = args[0]
	runner.args = args[1:]

	if runner.pm == nil {
		runner.pm = packmanager.G(ctx)
	}

	pm, compatible, err := runner.pm.IsCompatible(ctx, runner.packName)
	if err == nil && compatible {
		runner.pm = pm
		return true, nil
	} else if err != nil {
		return false, err
	}

	return false, nil
}

// Prepare implements Runner.
func (runner *runnerPackage) Prepare(ctx context.Context, opts *Run, machine *machineapi.Machine, args ...string) error {
	// First try the local cache of the catalog
	packs, err := runner.pm.Catalog(ctx,
		packmanager.WithTypes(unikraft.ComponentTypeApp),
		packmanager.WithName(runner.packName),
		packmanager.WithCache(true),
	)
	if err != nil {
		return err
	}

	if len(packs) > 1 {
		return fmt.Errorf("could not determine what to run: too many options")
	} else if len(packs) == 0 {
		// Second, try accessing the remote catalog
		packs, err = runner.pm.Catalog(ctx,
			packmanager.WithTypes(unikraft.ComponentTypeApp),
			packmanager.WithName(runner.packName),
			packmanager.WithCache(false),
		)
		if err != nil {
			return err
		}

		if len(packs) > 1 {
			return fmt.Errorf("could not determine what to run: too many options")
		} else if len(packs) == 0 {
			return fmt.Errorf("not found: %s", runner.packName)
		}
	}

	// Pre-emptively prepare the UID so that we can extract the kernel to the
	// defined state directory.
	machine.ObjectMeta.UID = uuid.NewUUID()
	machine.Status.StateDir = filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, string(machine.ObjectMeta.UID))
	if err := os.MkdirAll(machine.Status.StateDir, fs.ModeSetgid|0o775); err != nil {
		return err
	}

	group, err := user.LookupGroup(config.G[config.KraftKit](ctx).UserGroup)
	if err == nil {
		gid, err := strconv.ParseInt(group.Gid, 10, 32)
		if err != nil {
			return fmt.Errorf("could not parse group ID for kraftkit: %w", err)
		}

		if err := os.Chown(machine.Status.StateDir, os.Getuid(), int(gid)); err != nil {
			return fmt.Errorf("could not change group ownership of machine state dir: %w", err)
		}
	} else {
		log.G(ctx).
			WithField("error", err).
			Warn("kraftkit group not found, falling back to current user")
	}

	// Clean up the package directory if an error occurs before returning.
	defer func() {
		if err != nil {
			os.RemoveAll(machine.Status.StateDir)
		}
	}()

	paramodel, err := paraprogress.NewParaProgress(
		ctx,
		[]*paraprogress.Process{paraprogress.NewProcess(
			fmt.Sprintf("pulling %s", runner.packName),
			func(ctx context.Context, w func(progress float64)) error {
				return packs[0].Pull(
					ctx,
					pack.WithPullProgressFunc(w),
					pack.WithPullWorkdir(machine.Status.StateDir),
					pack.WithPullPlatform(opts.platform.String()),
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

	machine.Spec.Architecture = targ.Architecture().Name()
	machine.Spec.Platform = targ.Platform().Name()
	machine.Spec.Kernel = fmt.Sprintf("%s://%s", runner.pm.Format(), runner.packName)
	machine.Spec.ApplicationArgs = runner.args

	// Set the path to the initramfs if present.
	if opts.InitRd == "" && targ.Initrd() != nil {
		machine.Status.InitrdPath = targ.Initrd().Output
	} else if len(opts.InitRd) > 0 {
		machine.Status.InitrdPath = opts.InitRd
	}

	// Use the symbolic debuggable kernel image?
	if opts.WithKernelDbg {
		machine.Status.KernelPath = targ.KernelDbg()
	} else {
		machine.Status.KernelPath = targ.Kernel()
	}

	return nil
}

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
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/uuid"
	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft"
	ukarch "kraftkit.sh/unikraft/arch"
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
func (runner *runnerPackage) Runnable(ctx context.Context, opts *RunOptions, args ...string) (bool, error) {
	if len(args) == 0 {
		return false, fmt.Errorf("no arguments supplied")
	}

	runner.packName = args[0]
	runner.args = args[1:]

	if runner.pm == nil {
		runner.pm = packmanager.G(ctx)
	}

	pm, compatible, err := runner.pm.IsCompatible(ctx,
		runner.packName,
		packmanager.WithArchitecture(opts.Architecture),
		packmanager.WithPlatform(opts.platform.String()),
		packmanager.WithUpdate(true),
	)
	if err == nil && compatible {
		runner.pm = pm
		return true, nil
	} else if err != nil {
		return false, err
	}

	return false, nil
}

// Prepare implements Runner.
func (runner *runnerPackage) Prepare(ctx context.Context, opts *RunOptions, machine *machineapi.Machine, args ...string) error {
	qopts := []packmanager.QueryOption{
		packmanager.WithTypes(unikraft.ComponentTypeApp),
		packmanager.WithName(runner.packName),
	}

	// First try the local cache of the catalog
	packs, err := runner.pm.Catalog(ctx, qopts...)
	if err != nil {
		return err
	}
	if err != nil {
		return fmt.Errorf("could not query catalog: %w", err)
	} else if len(packs) == 0 {
		log.G(ctx).Debug("no local packages detected")

		// Try again with a remote update request.
		qopts = append(qopts, packmanager.WithUpdate(true))

		parallel := !config.G[config.KraftKit](ctx).NoParallel
		norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

		treemodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(parallel),
				processtree.WithRenderer(norender),
				processtree.WithFailFast(true),
				processtree.WithHideOnSuccess(true),
			},
			processtree.NewProcessTreeItem(
				"searching...", "",
				func(ctx context.Context) error {
					packs, err = runner.pm.Catalog(ctx, qopts...)
					if err != nil {
						return err
					}

					return nil
				},
			),
		)
		if err != nil {
			return err
		}
		if err := treemodel.Start(); err != nil {
			return fmt.Errorf("could not complete search: %v", err)
		}
		if len(packs) == 0 {
			return fmt.Errorf("coud not find runtime '%s'", runner.packName)
		}
	}

	if len(packs) > 1 && (opts.Architecture == "" || opts.platform == "") {
		// At this point, we have queried the registry without asking for the
		// platform and architecture and received multiple options.  Re-query the
		// catalog with the host architecture and platform.

		if opts.Architecture == "" {
			opts.Architecture, err = ukarch.HostArchitecture()
			if err != nil {
				return fmt.Errorf("could not get host architecture: %w", err)
			}
		}
		if opts.platform == "" {
			opts.platform, _, err = platform.Detect(ctx)
			if err != nil {
				return fmt.Errorf("could not get host platform: %w", err)
			}
		}

		for _, p := range packs {
			pt := p.(target.Target)
			if pt.Architecture().String() == opts.Architecture && pt.Platform().String() == opts.platform.String() {
				packs = []pack.Package{p}
				break
			}
		}

		if len(packs) != 1 {
			return fmt.Errorf("coud not find runtime '%s'", runner.packName)
		}

		log.G(ctx).
			WithField("arch", opts.Architecture).
			WithField("plat", opts.platform.String()).
			Info("using")
	}

	// Pre-emptively prepare the UID so that we can extract the kernel to the
	// defined state directory.
	machine.ObjectMeta.UID = uuid.NewUUID()
	machine.Status.StateDir = filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, string(machine.ObjectMeta.UID))
	if err := os.MkdirAll(machine.Status.StateDir, fs.ModeSetgid|0o775); err != nil {
		return err
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
	var ramfs initrd.Initrd
	if opts.Rootfs == "" && targ.Initrd() != nil {
		ramfs = targ.Initrd()
	} else if len(opts.Rootfs) > 0 {
		ramfs, err = initrd.New(ctx, opts.Rootfs)
		if err != nil {
			return err
		}
	}
	if ramfs != nil {
		machine.Status.InitrdPath, err = ramfs.Build(ctx)
		if err != nil {
			return err
		}
	}

	// Use the symbolic debuggable kernel image?
	if opts.WithKernelDbg {
		machine.Status.KernelPath = targ.KernelDbg()
	} else {
		machine.Status.KernelPath = targ.Kernel()
	}

	return nil
}

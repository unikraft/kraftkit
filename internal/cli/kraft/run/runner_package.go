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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	volumeapi "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/target"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
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
	return fmt.Sprintf("run the %s '%s' package and ignore cwd", runner.pm.Format(), runner.packName)
}

// Name implements Runner.
func (runner *runnerPackage) Name() string {
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

	// If the pack name is a file or directory, then do not proceed as these
	// cases are handled directly by other runners.
	if _, err := os.Stat(runner.packName); err == nil {
		return false, fmt.Errorf("arguments represent a file or directory")
	}

	runner.args = args[1:]

	if runner.pm == nil {
		runner.pm = packmanager.G(ctx)
	}

	pm, compatible, err := runner.pm.IsCompatible(ctx,
		runner.packName,
		packmanager.WithArchitecture(opts.Architecture),
		packmanager.WithPlatform(opts.platform.String()),
		packmanager.WithRemote(true),
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
	parallel := !config.G[config.KraftKit](ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	qopts := []packmanager.QueryOption{
		packmanager.WithTypes(unikraft.ComponentTypeApp),
		packmanager.WithName(runner.packName),
	}

	if len(opts.Architecture) > 0 {
		qopts = append(qopts, packmanager.WithArchitecture(opts.Architecture))
	}
	if len(opts.Platform) > 0 {
		qopts = append(qopts, packmanager.WithPlatform(opts.Platform))
	}

	// First try the local cache of the catalog
	var packs []pack.Package

	treemodel, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(parallel),
			processtree.WithRenderer(norender),
			processtree.WithFailFast(true),
			processtree.WithHideOnSuccess(true),
		},
		processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s", runner.packName), "",
			func(ctx context.Context) error {
				var err error
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

	if err != nil {
		return fmt.Errorf("could not query catalog: %w", err)
	} else if len(packs) == 0 {
		log.G(ctx).Debug("no local packages detected")

		// Try again with a remote update request.
		qopts = append(qopts, packmanager.WithRemote(true))

		treemodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(parallel),
				processtree.WithRenderer(norender),
				processtree.WithFailFast(true),
				processtree.WithHideOnSuccess(true),
			},
			processtree.NewProcessTreeItem(
				fmt.Sprintf("finding %s", runner.packName), "",
				func(ctx context.Context) error {
					var err error
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
	}

	var selected pack.Package

	if len(packs) == 1 {
		selected = packs[0]
	} else {
		found := []pack.Package{}

		for _, p := range packs {
			pt := p.(target.Target)
			if pt.Architecture().String() == opts.Architecture && pt.Platform().String() == opts.Platform {
				found = append(found, p)
			}
		}

		// Could not find a package that matches the desired architecture and platform.
		if len(found) == 0 {
			if !config.G[config.KraftKit](ctx).NoPrompt {
				log.G(ctx).Warnf("could not find package '%s' based on %s/%s", runner.packName, opts.Platform, opts.Architecture)
				p, err := selection.Select[pack.Package]("select alternative package with same name to continue", packs...)
				if err != nil {
					return fmt.Errorf("could not select package: %w", err)
				}

				selected = *p
			} else {
				return fmt.Errorf("could not find package '%s' based on %s/%s but %d others found but prompting has been disabled", runner.packName, opts.Platform, opts.Architecture, len(packs))
			}
		} else if len(found) == 1 {
			selected = found[0]
		} else {
			if !config.G[config.KraftKit](ctx).NoPrompt {
				log.G(ctx).Infof("found %d packages named '%s' based on %s/%s", len(found), runner.packName, opts.Platform, opts.Architecture)
				p, err := selection.Select[pack.Package]("select package to continue", found...)
				if err != nil {
					return fmt.Errorf("could not select package: %w", err)
				}

				selected = *p
			} else {
				return fmt.Errorf("found %d packages named '%s' based on %s/%s but prompting has been disabled", len(found), runner.packName, opts.Platform, opts.Architecture)
			}
		}
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

	if exists, _, err := selected.PulledAt(ctx); !exists || err != nil {
		paramodel, err := paraprogress.NewParaProgress(
			ctx,
			[]*paraprogress.Process{paraprogress.NewProcess(
				fmt.Sprintf("pulling %s", runner.packName),
				func(ctx context.Context, prompt func(), w func(progress float64)) error {
					return selected.Pull(
						ctx,
						pack.WithPullProgressFunc(w),
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
	}

	if err := selected.Unpack(
		ctx,
		machine.Status.StateDir,
	); err != nil {
		return fmt.Errorf("unpacking the image: %w", err)
	}

	// Crucially, the catalog should return an interface that also implements
	// target.Target.  This demonstrates that the implementing package can
	// resolve application kernels.
	targ, ok := selected.(target.Target)
	if !ok {
		return fmt.Errorf("package does not convert to target")
	}

	log.G(ctx).
		WithField("arch", targ.Architecture().Name()).
		WithField("plat", opts.platform.String()).
		Info("using")

	machine.Spec.Architecture = targ.Architecture().Name()
	machine.Spec.Platform = targ.Platform().Name()
	machine.Spec.Kernel = fmt.Sprintf("%s://%s", runner.pm.Format(), runner.packName)

	// If no arguments have been specified, use the ones which are default and
	// that have been included in the package.
	if len(runner.args) == 0 {
		runner.args = targ.Command()
	}

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

	// If automounting is enabled, and an initramfs is provided, set it as a
	// volume if a initram has been provided.
	if targ.KConfig().AnyYes(
		"CONFIG_LIBVFSCORE_FSTAB", // Deprecated
		"CONFIG_LIBVFSCORE_AUTOMOUNT_UP",
	) && (len(machine.Status.InitrdPath) > 0 || len(opts.Rootfs) > 0) {
		machine.Spec.Volumes = append(machine.Spec.Volumes, volumeapi.Volume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "fs0",
			},
			Spec: volumeapi.VolumeSpec{
				Driver:      "initrd",
				Destination: "/",
			},
		})
	}

	switch v := selected.Metadata().(type) {
	case *ocispec.Image:
		if machine.Spec.Env == nil {
			machine.Spec.Env = make(map[string]string)
		}

		for _, env := range v.Config.Env {
			k, v, ok := strings.Cut(env, "=")
			if !ok {
				continue
			}

			machine.Spec.Env[k] = v
		}
	default:
	}

	return nil
}

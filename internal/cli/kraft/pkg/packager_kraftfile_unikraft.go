// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package pkg

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/mattn/go-shellwords"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/multiselect"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/target"
)

type packagerKraftfileUnikraft struct{}

// String implements fmt.Stringer.
func (p *packagerKraftfileUnikraft) String() string {
	return "kraftfile-unikraft"
}

// Buildable implements packager.
func (p *packagerKraftfileUnikraft) Packagable(ctx context.Context, opts *PkgOptions, args ...string) (bool, error) {
	if opts.Project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.Project.Unikraft(ctx) == nil {
		return false, fmt.Errorf("cannot package without unikraft core specification")
	}

	if opts.Project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.Project.Rootfs()
	}

	return true, nil
}

// Build implements packager.
func (p *packagerKraftfileUnikraft) Pack(ctx context.Context, opts *PkgOptions, args ...string) ([]pack.Package, error) {
	var err error

	var tree []*processtree.ProcessTreeItem

	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	selected := opts.Project.Targets()
	if len(opts.Target) > 0 || len(opts.Architecture) > 0 || len(opts.Platform) > 0 {
		selected = target.Filter(opts.Project.Targets(), opts.Architecture, opts.Platform, opts.Target)
	}

	if len(selected) > 1 && !config.G[config.KraftKit](ctx).NoPrompt {
		// Remove targets which do not have a compiled kernel.
		targets := slices.DeleteFunc(opts.Project.Targets(), func(targ target.Target) bool {
			_, err := os.Stat(targ.Kernel())
			return err != nil
		})

		if len(targets) == 0 {
			return nil, fmt.Errorf("no targets with a compiled kernel found")
		} else if len(targets) == 1 {
			selected = targets
		} else {
			selected, err = multiselect.MultiSelect[target.Target]("select built kernel to package", targets...)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("nothing selected to package")
	}

	i := 0

	var result []pack.Package

	for _, targ := range selected {
		var cmds []string
		var envs []string
		rootfs := opts.Rootfs

		// Reset the rootfs, such that it is not packaged as an initrd if it is
		// already embedded inside of the kernel.
		if opts.Project.KConfig().AnyYes(
			"CONFIG_LIBVFSCORE_ROOTFS_EINITRD", // Deprecated
			"CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD",
			"CONFIG_LIBVFSCORE_AUTOMOUNT_CI_EINITRD",
		) {
			rootfs = ""
		} else {
			if rootfs, cmds, envs, err = utils.BuildRootfs(ctx, opts.Workdir, rootfs, opts.Compress, targ.Architecture().String()); err != nil {
				return nil, fmt.Errorf("could not build rootfs: %w", err)
			}
		}

		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ
		baseopts := opts.packopts
		name := "packaging " + targ.Name() + " (" + opts.Format + ")"

		if envs != nil {
			opts.Env = append(opts.Env, envs...)
		}

		// If no arguments have been specified, use the ones which are default and
		// that have been included in the package.
		if len(opts.Args) == 0 {
			if len(opts.Project.Command()) > 0 {
				opts.Args = opts.Project.Command()
			} else if len(targ.Command()) > 0 {
				opts.Args = targ.Command()
			} else if cmds != nil {
				opts.Args = cmds
			}
		}

		cmdShellArgs, err := shellwords.Parse(strings.Join(opts.Args, " "))
		if err != nil {
			return nil, err
		}

		// When i > 0, we have already applied the merge strategy.  Now, for all
		// targets, we actually do wish to merge these because they are part of
		// the same execution lifecycle.
		if i > 0 {
			baseopts = []packmanager.PackOption{
				packmanager.PackMergeStrategy(packmanager.StrategyMerge),
			}
		}

		tree = append(tree, processtree.NewProcessTreeItem(
			name,
			targ.Architecture().Name()+"/"+targ.Platform().Name(),
			func(ctx context.Context) error {
				popts := append(baseopts,
					packmanager.PackArgs(cmdShellArgs...),
					packmanager.PackInitrd(rootfs),
					packmanager.PackKConfig(!opts.NoKConfig),
					packmanager.PackName(opts.Name),
					packmanager.PackOutput(opts.Output),
				)

				if ukversion, ok := targ.KConfig().Get(unikraft.UK_FULLVERSION); ok {
					popts = append(popts,
						packmanager.PackWithKernelVersion(ukversion.Value),
					)
				}

				envs := opts.aggregateEnvs()
				if len(envs) > 0 {
					popts = append(popts, packmanager.PackWithEnvs(envs))
				} else if len(opts.Env) > 0 {
					popts = append(popts, packmanager.PackWithEnvs(opts.Env))
				}

				more, err := opts.pm.Pack(ctx, targ, popts...)
				if err != nil {
					return err
				}

				result = append(result, more...)

				return nil
			},
		))

		i++
	}

	if len(tree) == 0 {
		switch true {
		case len(opts.Target) > 0:
			return nil, fmt.Errorf("no matching targets found for: %s", opts.Target)
		case len(opts.Architecture) > 0 && len(opts.Platform) == 0:
			return nil, fmt.Errorf("no matching targets found for architecture: %s", opts.Architecture)
		case len(opts.Architecture) == 0 && len(opts.Platform) > 0:
			return nil, fmt.Errorf("no matching targets found for platform: %s", opts.Platform)
		default:
			return nil, fmt.Errorf("no matching targets found for: %s/%s", opts.Platform, opts.Architecture)
		}
	}

	model, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithRenderer(norender),
		},
		tree...,
	)
	if err != nil {
		return nil, err
	}

	if err := model.Start(); err != nil {
		return nil, err
	}

	return result, nil
}

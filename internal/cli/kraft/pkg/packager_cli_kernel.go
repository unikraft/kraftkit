// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package pkg

import (
	"context"
	"fmt"
	"strings"

	"github.com/mattn/go-shellwords"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

type packagerCliKernel struct{}

// String implements fmt.Stringer.
func (p *packagerCliKernel) String() string {
	return "cli-kernel"
}

// Packagable implements packager.
func (p *packagerCliKernel) Packagable(ctx context.Context, opts *PkgOptions, args ...string) (bool, error) {
	if len(opts.Kernel) > 0 && len(opts.Architecture) > 0 && len(opts.Platform) > 0 {
		return true, nil
	}

	return false, fmt.Errorf("cannot package without path to -k|-kernel, -m|--arch and -p|--plat")
}

// Pack implements packager.
func (p *packagerCliKernel) Pack(ctx context.Context, opts *PkgOptions, args ...string) ([]pack.Package, error) {
	ac, err := arch.NewArchitectureFromOptions(
		arch.WithName(opts.Architecture),
	)
	if err != nil {
		return nil, fmt.Errorf("could not prepare architecture: %w", err)
	}

	pc, err := plat.NewPlatformFromOptions(
		plat.WithName(opts.Platform),
	)
	if err != nil {
		return nil, fmt.Errorf("could not prepare architecture: %w", err)
	}

	targ, err := target.NewTargetFromOptions(
		target.WithArchitecture(ac),
		target.WithPlatform(pc),
		target.WithKernel(opts.Kernel),
		target.WithCommand(opts.Args),
	)
	if err != nil {
		return nil, fmt.Errorf("could not prepare phony target: %w", err)
	}

	if opts.Rootfs, err = utils.BuildRootfs(ctx, opts.Workdir, opts.Rootfs, targ); err != nil {
		return nil, fmt.Errorf("could not build rootfs: %w", err)
	}

	cmdShellArgs, err := shellwords.Parse(strings.Join(opts.Args, " "))
	if err != nil {
		return nil, err
	}

	var result []pack.Package
	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	model, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithRenderer(norender),
		},

		processtree.NewProcessTreeItem(
			"packaging "+opts.Name+" ("+opts.Format+")",
			opts.Platform+"/"+opts.Architecture,
			func(ctx context.Context) error {
				popts := append(opts.packopts,
					packmanager.PackArgs(cmdShellArgs...),
					packmanager.PackInitrd(opts.Rootfs),
					packmanager.PackKConfig(!opts.NoKConfig),
					packmanager.PackName(opts.Name),
					packmanager.PackOutput(opts.Output),
				)

				more, err := opts.pm.Pack(ctx, targ, popts...)
				if err != nil {
					return err
				}

				result = append(result, more...)

				return nil
			},
		),
	)
	if err != nil {
		return nil, err
	}

	if err := model.Start(); err != nil {
		return nil, err
	}

	return result, nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"fmt"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft/target"
)

type builderKraftfileRuntime struct{}

// String implements fmt.Stringer.
func (build *builderKraftfileRuntime) String() string {
	return "kraftfile-runtime"
}

// Buildable implements builder.
func (build *builderKraftfileRuntime) Buildable(ctx context.Context, opts *BuildOptions, args ...string) (bool, error) {
	if opts.NoRootfs {
		return false, fmt.Errorf("building rootfs disabled")
	}

	if opts.Project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.Project.Runtime() == nil {
		return false, fmt.Errorf("cannot package without unikraft core specification")
	}

	if opts.Project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.Project.Rootfs()
	}

	return true, nil
}

func (*builderKraftfileRuntime) Prepare(ctx context.Context, opts *BuildOptions, _ ...string) error {
	var (
		selected *pack.Package
		packs    []pack.Package
		kconfigs []string
		err      error
	)

	name := opts.Project.Runtime().Name()
	if opts.Platform == "kraftcloud" || (opts.Project.Runtime().Platform() != nil && opts.Project.Runtime().Platform().Name() == "kraftcloud") {
		name = utils.RewrapAsKraftCloudPackage(name)
	}

	qopts := []packmanager.QueryOption{
		packmanager.WithName(name),
		packmanager.WithVersion(opts.Project.Runtime().Version()),
	}

	treemodel, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithRenderer(
				log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
			),
			processtree.WithFailFast(true),
			processtree.WithHideOnSuccess(true),
		},
		processtree.NewProcessTreeItem(
			fmt.Sprintf(
				"searching for %s:%s",
				name,
				opts.Project.Runtime().Version(),
			),
			"",
			func(ctx context.Context) error {
				qopts = append(qopts,
					packmanager.WithArchitecture(opts.Architecture),
					packmanager.WithPlatform(opts.Platform),
					packmanager.WithKConfig(kconfigs),
				)

				packs, err = packmanager.G(ctx).Catalog(ctx, append(qopts, packmanager.WithRemote(false))...)
				if err != nil {
					return fmt.Errorf("could not query catalog: %w", err)
				} else if len(packs) == 0 {
					// Try again with a remote update request.  Save this to qopts in case we
					// need to call `Catalog` again.
					packs, err = packmanager.G(ctx).Catalog(ctx, append(qopts, packmanager.WithRemote(true))...)
					if err != nil {
						return fmt.Errorf("could not query catalog: %w", err)
					}
				}

				return nil
			},
		),
	)
	if err != nil {
		return err
	}

	if err := treemodel.Start(); err != nil {
		return err
	}

	if len(packs) == 0 {
		return fmt.Errorf(
			"could not find runtime '%s:%s'",
			opts.Project.Runtime().Name(),
			opts.Project.Runtime().Version(),
		)
	} else if len(packs) == 1 {
		selected = &packs[0]
	} else if len(packs) > 1 {
		// If a target has been previously selected, we can use this to filter the
		// returned list of packages based on its platform and architecture.

		selected, err = selection.Select("multiple runtimes available", packs...)
		if err != nil {
			return err
		}
	}

	targ := (*selected).(target.Target)
	opts.Target = &targ

	return nil
}

func (*builderKraftfileRuntime) Build(_ context.Context, _ *BuildOptions, _ ...string) error {
	return nil
}

func (*builderKraftfileRuntime) Statistics(ctx context.Context, opts *BuildOptions, args ...string) error {
	return fmt.Errorf("cannot calculate statistics of pre-built unikernel runtime")
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package pkg

import (
	"context"
	"fmt"

	"github.com/mattn/go-shellwords"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/multiselect"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

type packagerKraftfileUnikraft struct{}

// String implements fmt.Stringer.
func (p *packagerKraftfileUnikraft) String() string {
	return "unikraft"
}

// Buildable implements packager.
func (p *packagerKraftfileUnikraft) Packagable(ctx context.Context, opts *Pkg, args ...string) (bool, error) {
	var err error

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Interpret the project directory
	opts.project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return false, err
	}

	if opts.project.Unikraft(ctx) == nil {
		return false, fmt.Errorf("cannot package without unikraft core specification")
	}

	return true, nil
}

// Build implements packager.
func (p *packagerKraftfileUnikraft) Pack(ctx context.Context, opts *Pkg, args ...string) error {
	var err error

	pm := packmanager.G(ctx)

	// Switch the package manager the desired format for this target
	pm, err = pm.From(pack.PackageFormat(opts.Format))
	if err != nil {
		return err
	}

	var tree []*processtree.ProcessTreeItem

	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	exists, err := pm.Catalog(ctx,
		packmanager.WithName(opts.Name),
	)
	baseopts := []packmanager.PackOption{}
	if err == nil && len(exists) >= 1 {
		if opts.strategy == packmanager.StrategyPrompt {
			strategy, err := selection.Select[packmanager.MergeStrategy](
				fmt.Sprintf("package '%s' already exists: how would you like to proceed?", opts.Name),
				packmanager.MergeStrategies()...,
			)
			if err != nil {
				return err
			}

			opts.strategy = *strategy
		}

		switch opts.strategy {
		case packmanager.StrategyExit:
			return fmt.Errorf("package already exists and merge strategy set to exit on conflict")

		// Set the merge strategy as an option that is then passed to the
		// package manager.
		default:
			baseopts = append(baseopts,
				packmanager.PackMergeStrategy(opts.strategy),
			)
		}
	} else {
		baseopts = append(baseopts,
			packmanager.PackMergeStrategy(packmanager.StrategyMerge),
		)
	}

	var selected []target.Target
	if len(opts.Target) > 0 || len(opts.Architecture) > 0 || len(opts.Platform) > 0 && !config.G[config.KraftKit](ctx).NoPrompt {
		selected = target.Filter(opts.project.Targets(), opts.Architecture, opts.Platform, opts.Target)
	} else {
		selected, err = multiselect.MultiSelect[target.Target]("select what to package", opts.project.Targets()...)
		if err != nil {
			return err
		}
	}

	if len(selected) == 0 {
		return fmt.Errorf("nothing selected to package")
	}

	i := 0

	for _, targ := range selected {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ
		baseopts := baseopts
		name := "packaging " + targ.Name() + " (" + opts.Format + ")"

		cmdShellArgs, err := shellwords.Parse(opts.Args)
		if err != nil {
			return err
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
					packmanager.PackInitrd(opts.Initrd),
					packmanager.PackKConfig(!opts.NoKConfig),
					packmanager.PackName(opts.Name),
					packmanager.PackOutput(opts.Output),
				)

				if ukversion, ok := targ.KConfig().Get(unikraft.UK_FULLVERSION); ok {
					popts = append(popts,
						packmanager.PackWithKernelVersion(ukversion.Value),
					)
				}

				if _, err := pm.Pack(ctx, targ, popts...); err != nil {
					return err
				}

				return nil
			},
		))

		i++
	}

	if len(tree) == 0 {
		switch true {
		case len(opts.Target) > 0:
			return fmt.Errorf("no matching targets found for: %s", opts.Target)
		case len(opts.Architecture) > 0 && len(opts.Platform) == 0:
			return fmt.Errorf("no matching targets found for architecture: %s", opts.Architecture)
		case len(opts.Architecture) == 0 && len(opts.Platform) > 0:
			return fmt.Errorf("no matching targets found for platform: %s", opts.Platform)
		default:
			return fmt.Errorf("no matching targets found for: %s/%s", opts.Platform, opts.Architecture)
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
		return err
	}

	return model.Start()
}

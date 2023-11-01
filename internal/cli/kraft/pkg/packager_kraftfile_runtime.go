// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package pkg

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-shellwords"
	"kraftkit.sh/config"
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

type packagerKraftfileRuntime struct{}

// String implements fmt.Stringer.
func (p *packagerKraftfileRuntime) String() string {
	return "runtime"
}

// Packagable implements packager.
func (p *packagerKraftfileRuntime) Packagable(ctx context.Context, opts *PkgOptions, args ...string) (bool, error) {
	if opts.Project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.Project.Runtime() == nil {
		return false, fmt.Errorf("cannot package without unikraft core specification")
	}

	return true, nil
}

// Pack implements packager.
func (p *packagerKraftfileRuntime) Pack(ctx context.Context, opts *PkgOptions, args ...string) ([]pack.Package, error) {
	var err error
	var targ target.Target

	targets := opts.Project.Targets()
	qopts := []packmanager.QueryOption{
		packmanager.WithName(opts.Project.Runtime().Name()),
		packmanager.WithVersion(opts.Project.Runtime().Version()),
	}

	if len(targets) == 1 {
		targ = targets[0]
	} else if len(targets) > 1 {
		// Filter project targets by any provided CLI options
		targets = target.Filter(
			targets,
			opts.Architecture,
			opts.Platform,
			opts.Target,
		)

		switch {
		case len(targets) == 0:
			return nil, fmt.Errorf("could not detect any project targets based on plat=\"%s\" arch=\"%s\"", opts.Platform, opts.Architecture)

		case len(targets) == 1:
			targ = targets[0]

		case config.G[config.KraftKit](ctx).NoPrompt && len(targets) > 1:
			return nil, fmt.Errorf("could not determine what to run based on provided CLI arguments")

		default:
			targ, err = target.Select(targets)
			if err != nil {
				return nil, fmt.Errorf("could not select target: %v", err)
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
		if opts.Platform != "" {
			qopts = append(qopts,
				packmanager.WithPlatform(opts.Platform),
			)
		}
		if opts.Architecture != "" {
			qopts = append(qopts,
				packmanager.WithArchitecture(opts.Architecture),
			)
		}
	}

	packs, err := opts.pm.Catalog(ctx, append(qopts, packmanager.WithUpdate(false))...)
	if err != nil {
		return nil, fmt.Errorf("could not query catalog: %w", err)
	} else if len(packs) == 0 {
		// Try again with a remote update request.  Save this to qopts in case we
		// need to call `Catalog` again.
		qopts = append(qopts, packmanager.WithUpdate(true))
		packs, err = opts.pm.Catalog(ctx, qopts...)
		if err != nil {
			return nil, fmt.Errorf("could not query catalog: %w", err)
		} else if len(packs) == 0 {
			return nil, fmt.Errorf("coud not find runtime '%s'", opts.Project.Runtime().Name())
		}
	}

	if len(packs) > 1 && targ == nil && (opts.Architecture == "" || opts.Platform == "") {
		// At this point, we have queried the registry without asking for the
		// platform and architecture and received multiple options.  Re-query the
		// catalog with the host architecture and platform.

		if opts.Architecture == "" {
			opts.Architecture, err = ukarch.HostArchitecture()
			if err != nil {
				return nil, fmt.Errorf("could not get host architecture: %w", err)
			}
		}
		if opts.Platform == "" {
			plat, _, err := platform.Detect(ctx)
			if err != nil {
				return nil, fmt.Errorf("could not get host platform: %w", err)
			}

			opts.Platform = plat.String()
		}

		qopts = append(qopts,
			packmanager.WithPlatform(opts.Platform),
			packmanager.WithArchitecture(opts.Architecture),
		)

		packs, err = opts.pm.Catalog(ctx, qopts...)
		if err != nil {
			return nil, fmt.Errorf("could not query catalog: %w", err)
		} else if len(packs) == 0 {
			return nil, fmt.Errorf("coud not find runtime '%s'", opts.Project.Runtime().Name())
		} else if len(packs) > 1 {
			return nil, fmt.Errorf("could not find runtime: too many options")
		}

		log.G(ctx).
			WithField("arch", opts.Architecture).
			WithField("plat", opts.Platform).
			Info("using")
	}

	// Create a temporary directory we can use to store the artifacts from
	// pulling and extracting the identified package.
	tempDir, err := os.MkdirTemp("", "kraft-pkg-")
	if err != nil {
		return nil, fmt.Errorf("could not create temporary directory: %w", err)
	}

	defer func() {
		os.RemoveAll(tempDir)
	}()

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
		return nil, err
	}

	if err := paramodel.Start(); err != nil {
		return nil, err
	}

	// Crucially, the catalog should return an interface that also implements
	// target.Target.  This demonstrates that the implementing package can
	// resolve application kernels.
	targ, ok := packs[0].(target.Target)
	if !ok {
		return nil, fmt.Errorf("package does not convert to target")
	}

	// If no arguments have been specified, use the ones which are default and
	// that have been included in the package.
	if len(opts.Args) == 0 {
		opts.Args = targ.Command()
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
			targ.Platform().Name()+"/"+targ.Architecture().Name(),
			func(ctx context.Context) error {
				popts := append(opts.packopts,
					packmanager.PackArgs(cmdShellArgs...),
					packmanager.PackInitrd(opts.Rootfs),
					packmanager.PackKConfig(!opts.NoKConfig),
					packmanager.PackName(opts.Name),
					packmanager.PackOutput(opts.Output),
				)

				if ukversion, ok := targ.KConfig().Get(unikraft.UK_FULLVERSION); ok {
					popts = append(popts,
						packmanager.PackWithKernelVersion(ukversion.Value),
					)
				}

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

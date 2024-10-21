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
	"kraftkit.sh/internal/cli/kraft/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/target"
)

type packagerKraftfileRuntime struct{}

// String implements fmt.Stringer.
func (p *packagerKraftfileRuntime) String() string {
	return "kraftfile-runtime"
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

	if opts.Project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.Project.Rootfs()
	}

	return true, nil
}

// Pack implements packager.
func (p *packagerKraftfileRuntime) Pack(ctx context.Context, opts *PkgOptions, args ...string) ([]pack.Package, error) {
	var err error
	var targ target.Target
	var runtimeName string

	if len(opts.Runtime) > 0 {
		runtimeName = opts.Runtime
	} else {
		runtimeName = opts.Project.Runtime().Name()
	}

	if opts.Platform == "kraftcloud" || (opts.Project.Runtime().Platform() != nil && opts.Project.Runtime().Platform().Name() == "kraftcloud") {
		runtimeName = utils.RewrapAsKraftCloudPackage(runtimeName)
	}

	targets := opts.Project.Targets()
	qopts := []packmanager.QueryOption{
		packmanager.WithName(runtimeName),
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

	var selected *pack.Package
	var packs []pack.Package
	var kconfigs []string

	if targ != nil {
		for _, kc := range targ.KConfig() {
			kconfigs = append(kconfigs, kc.String())
		}

		if opts.Platform == "" {
			opts.Platform = targ.Platform().Name()
		}
		if opts.Architecture == "" {
			opts.Architecture = targ.Architecture().Name()
		}
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
				runtimeName,
				opts.Project.Runtime().Version(),
			),
			"",
			func(ctx context.Context) error {
				qopts = append(qopts,
					packmanager.WithArchitecture(opts.Architecture),
					packmanager.WithPlatform(opts.Platform),
					packmanager.WithKConfig(kconfigs),
				)

				packs, err = opts.pm.Catalog(ctx, append(qopts, packmanager.WithRemote(false))...)
				if err != nil {
					return fmt.Errorf("could not query catalog: %w", err)
				} else if len(packs) == 0 {
					// Try again with a remote update request.  Save this to qopts in case we
					// need to call `Catalog` again.
					packs, err = opts.pm.Catalog(ctx, append(qopts, packmanager.WithRemote(true))...)
					if err != nil {
						return fmt.Errorf("could not query catalog: %w", err)
					}
				}

				return nil
			},
		),
	)
	if err != nil {
		return nil, err
	}

	if err := treemodel.Start(); err != nil {
		return nil, err
	}

	if len(packs) == 0 {
		if len(opts.Platform) > 0 && len(opts.Architecture) > 0 {
			return nil, fmt.Errorf(
				"could not find runtime '%s:%s' (%s/%s)",
				opts.Project.Runtime().Name(),
				opts.Project.Runtime().Version(),
				opts.Platform,
				opts.Architecture,
			)
		} else if len(opts.Architecture) > 0 {
			return nil, fmt.Errorf(
				"could not find runtime '%s:%s' with '%s' architecture",
				opts.Project.Runtime().Name(),
				opts.Project.Runtime().Version(),
				opts.Architecture,
			)
		} else if len(opts.Platform) > 0 {
			return nil, fmt.Errorf(
				"could not find runtime '%s:%s' with '%s' platform",
				opts.Project.Runtime().Name(),
				opts.Project.Runtime().Version(),
				opts.Platform,
			)
		} else {
			return nil, fmt.Errorf(
				"could not find runtime %s:%s",
				opts.Project.Runtime().Name(),
				opts.Project.Runtime().Version(),
			)
		}
	} else if len(packs) == 1 {
		selected = &packs[0]
	} else if len(packs) > 1 {
		// If a target has been previously selected, we can use this to filter the
		// returned list of packages based on its platform and architecture.
		if targ != nil {
			found := []pack.Package{}

			for _, p := range packs {
				pt := p.(target.Target)
				if pt.Architecture().String() == opts.Architecture && pt.Platform().String() == opts.Platform {
					found = append(found, p)
				}
			}

			// Could not find a package that matches the desired architecture and
			// platform, prompt with available set of packages.
			if len(found) == 0 {
				if !config.G[config.KraftKit](ctx).NoPrompt {
					log.G(ctx).Warnf("could not find package '%s:%s' based on %s/%s", runtimeName, opts.Project.Runtime().Version(), opts.Platform, opts.Architecture)
					p, err := selection.Select[pack.Package]("select alternative package with same name to continue", packs...)
					if err != nil {
						return nil, fmt.Errorf("could not select package: %w", err)
					}

					selected = p
				} else {
					return nil, fmt.Errorf("could not find package '%s:%s' based on %s/%s but %d others found but prompting has been disabled", runtimeName, opts.Project.Runtime().Version(), opts.Platform, opts.Architecture, len(packs))
				}
			} else if len(found) == 1 {
				selected = &found[0]
			} else { // > 1
				if !config.G[config.KraftKit](ctx).NoPrompt {
					log.G(ctx).Infof("found %d packages named '%s:%s' based on %s/%s", len(found), runtimeName, opts.Project.Runtime().Version(), opts.Platform, opts.Architecture)
					p, err := selection.Select[pack.Package]("select package to continue", found...)
					if err != nil {
						return nil, fmt.Errorf("could not select package: %w", err)
					}

					selected = p
				} else {
					return nil, fmt.Errorf("found %d packages named '%s:%s' based on %s/%s but prompting has been disabled", len(found), runtimeName, opts.Project.Runtime().Version(), opts.Platform, opts.Architecture)
				}
			}
		} else {
			selected, err = selection.Select[pack.Package]("multiple runtimes available", packs...)
			if err != nil {
				return nil, err
			}
		}
	}

	runtime := *selected
	pulled, _, _ := runtime.PulledAt(ctx)

	// Temporarily save the runtime package.
	if err := runtime.Save(ctx); err != nil {
		return nil, fmt.Errorf("could not save runtime package: %w", err)
	}

	// Remove the cached runtime package reference if it was not previously
	// pulled.
	if !pulled && opts.NoPull {
		defer func() {
			if err := runtime.Delete(ctx); err != nil {
				log.G(ctx).Debugf("could not delete intermediate runtime package: %s", err.Error())
			}
		}()
	}

	if !pulled && !opts.NoPull {
		paramodel, err := paraprogress.NewParaProgress(
			ctx,
			[]*paraprogress.Process{paraprogress.NewProcess(
				fmt.Sprintf("pulling %s", runtime.String()),
				func(ctx context.Context, w func(progress float64)) error {
					popts := []pack.PullOption{}
					if log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) == log.FANCY {
						popts = append(popts, pack.WithPullProgressFunc(w))
					}

					return runtime.Pull(
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

	// Crucially, the catalog should return an interface that also implements
	// target.Target.  This demonstrates that the implementing package can
	// resolve application kernels.
	targ, ok := runtime.(target.Target)
	if !ok {
		return nil, fmt.Errorf("package does not convert to target")
	}

	var cmds []string
	var envs []string
	if opts.Rootfs, cmds, envs, err = utils.BuildRootfs(ctx, opts.Workdir, opts.Rootfs, opts.Compress, targ.Architecture().String()); err != nil {
		return nil, fmt.Errorf("could not build rootfs: %w", err)
	}

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

	args = []string{}

	// Only parse arguments if they have been provided.
	if len(opts.Args) > 0 {
		args, err = shellwords.Parse(fmt.Sprintf("'%s'", strings.Join(opts.Args, "' '")))
		if err != nil {
			return nil, err
		}
	}

	labels := opts.Project.Labels()
	if len(opts.Labels) > 0 {
		for _, label := range opts.Labels {
			kv := strings.SplitN(label, "=", 2)
			if len(kv) != 2 {
				return nil, fmt.Errorf("invalid label format: %s", label)
			}

			labels[kv[0]] = kv[1]
		}
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
			"packaging "+opts.Name,
			targ.Platform().Name()+"/"+targ.Architecture().Name(),
			func(ctx context.Context) error {
				popts := append(opts.packopts,
					packmanager.PackArgs(args...),
					packmanager.PackInitrd(opts.Rootfs),
					packmanager.PackKConfig(!opts.NoKConfig),
					packmanager.PackName(opts.Name),
					packmanager.PackOutput(opts.Output),
					packmanager.PackLabels(labels),
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

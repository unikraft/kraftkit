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

	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/cli/kraft/utils"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

type packagerKraftfileRuntime struct {
	name    string
	version string
	target  target.Target

	// Packaging options
	kernel       string
	kconfig      kconfig.KeyValueMap
	args         []string
	env          []string
	roms         []string
	rootfs       initrd.Initrd
	architecture arch.Architecture
	platform     plat.Platform
}

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

	if opts.Project.Runtime() == nil && len(opts.Project.Rootfs()) == 0 && len(opts.Project.Roms()) == 0 {
		return false, fmt.Errorf("cannot package without any of runtime, rootfs or roms")
	}

	if opts.Project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.Project.Rootfs()
	}

	if opts.Project.InitrdFsType().String() != "" && opts.RootfsType == "" {
		opts.RootfsType = opts.Project.InitrdFsType()
	}

	return true, nil
}

// Pack implements packager.
func (p *packagerKraftfileRuntime) Pack(ctx context.Context, opts *PkgOptions, args ...string) ([]pack.Package, error) {
	var err error

	if len(opts.Runtime) > 0 {
		var ok bool
		p.name, p.version, ok = strings.Cut(opts.Runtime, ":")
		if !ok {
			p.version = "latest"
		}
	} else if opts.Project != nil && opts.Project.Runtime() != nil {
		p.name = opts.Project.Runtime().Name()
	} else if opts.Name != "" {
		var ok bool
		p.name, p.version, ok = strings.Cut(opts.Name, ":")
		if !ok {
			p.version = "latest"
		}
	} else {
		return nil, fmt.Errorf("no name specified: ")
	}

	if opts.Platform == "kraftcloud" || (opts.Project != nil && opts.Project.Runtime() != nil && opts.Project.Runtime().Platform() != nil && opts.Project.Runtime().Platform().Name() == "kraftcloud") {
		p.name = utils.RewrapAsKraftCloudPackage(p.name)
	}

	var targets []target.Target

	if opts.Project != nil {
		targets = opts.Project.Targets()

		if opts.Project.Runtime() != nil {
			p.version = opts.Project.Runtime().Version()
		}
	}

	qopts := []packmanager.QueryOption{
		packmanager.WithName(p.name),
		packmanager.WithVersion(p.version),
	}

	if len(targets) == 1 {
		p.target = targets[0]
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
			p.target = targets[0]

		case config.G[config.KraftKit](ctx).NoPrompt && len(targets) > 1:
			return nil, fmt.Errorf("could not determine what to run based on provided CLI arguments")

		default:
			p.target, err = target.Select(targets)
			if err != nil {
				return nil, fmt.Errorf("could not select target: %v", err)
			}
		}
	}

	var selected *pack.Package
	var packs []pack.Package
	var kconfigs []string

	if p.target != nil {
		for _, kc := range p.target.KConfig() {
			kconfigs = append(kconfigs, kc.String())
		}

		if opts.Platform == "" {
			opts.Platform = p.target.Platform().Name()
		}
		if opts.Architecture == "" {
			opts.Architecture = p.target.Architecture().Name()
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
				p.name,
				p.version,
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
				} else if len(packs) == 0 && !opts.NoPull {
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

	if len(packs) == 0 && !opts.NoKernel {
		if len(opts.Platform) > 0 && len(opts.Architecture) > 0 {
			return nil, fmt.Errorf(
				"could not find runtime '%s:%s' (%s/%s)",
				p.name,
				p.version,
				opts.Platform,
				opts.Architecture,
			)
		} else if len(opts.Architecture) > 0 {
			return nil, fmt.Errorf(
				"could not find runtime '%s:%s' with '%s' architecture",
				p.name,
				p.version,
				opts.Architecture,
			)
		} else if len(opts.Platform) > 0 {
			return nil, fmt.Errorf(
				"could not find runtime '%s:%s' with '%s' platform",
				p.name,
				p.version,
				opts.Platform,
			)
		} else {
			return nil, fmt.Errorf(
				"could not find runtime %s:%s",
				p.name,
				p.version,
			)
		}
	} else if len(packs) == 1 {
		selected = &packs[0]
	} else if len(packs) > 1 {
		// If a target has been previously selected, we can use this to filter the
		// returned list of packages based on its platform and architecture.
		if p.target != nil {
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
					log.G(ctx).Warnf("could not find package '%s:%s' based on %s/%s", p.name, opts.Project.Runtime().Version(), opts.Platform, opts.Architecture)
					p, err := selection.Select[pack.Package]("select alternative package with same name to continue", packs...)
					if err != nil {
						return nil, fmt.Errorf("could not select package: %w", err)
					}

					selected = p
				} else {
					return nil, fmt.Errorf("could not find package '%s:%s' based on %s/%s but %d others found but prompting has been disabled", p.name, opts.Project.Runtime().Version(), opts.Platform, opts.Architecture, len(packs))
				}
			} else if len(found) == 1 {
				selected = &found[0]
			} else { // > 1
				if !config.G[config.KraftKit](ctx).NoPrompt {
					log.G(ctx).Infof("found %d packages named '%s:%s' based on %s/%s", len(found), p.name, opts.Project.Runtime().Version(), opts.Platform, opts.Architecture)
					p, err := selection.Select[pack.Package]("select package to continue", found...)
					if err != nil {
						return nil, fmt.Errorf("could not select package: %w", err)
					}

					selected = p
				} else {
					return nil, fmt.Errorf("found %d packages named '%s:%s' based on %s/%s but prompting has been disabled", len(found), p.name, opts.Project.Runtime().Version(), opts.Platform, opts.Architecture)
				}
			}
		} else {
			selected, err = selection.Select[pack.Package]("multiple runtimes available", packs...)
			if err != nil {
				return nil, err
			}
		}
	}

	if selected != nil {
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
					log.G(ctx).Tracef("could not delete intermediate runtime package: %s", err.Error())
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
			if err := os.RemoveAll(tempDir); err != nil {
				log.G(ctx).Debugf("could not delete temporary directory: %s", err.Error())
			}
		}()

		// Crucially, the catalog should return an interface that also implements
		// target.Target.  This demonstrates that the implementing package can
		// resolve application kernels.
		var ok bool
		p.target, ok = runtime.(target.Target)
		if !ok {
			return nil, fmt.Errorf("package does not convert to target")
		}

		opts.Platform = p.target.Platform().Name()
		opts.Architecture = p.target.Architecture().Name()
		p.kernel = p.target.Kernel()
		p.kconfig = p.target.KConfig()
		p.architecture = p.target.Architecture()
		p.platform = p.target.Platform()
	} else {
		if len(opts.Platform) == 0 {
			return nil, fmt.Errorf("no platform specified: required when no runtime is specified")
		}
		if len(opts.Architecture) == 0 {
			return nil, fmt.Errorf("no architecture specified: required when no runtime is specified")
		}

		p.architecture = arch.NewArchitectureFromOptions(
			arch.WithName(opts.Architecture),
		)
		p.platform = plat.NewPlatformFromOptions(
			plat.WithName(opts.Platform),
		)

		log.G(ctx).Warn("no kernel detected: packaging without - this may produce unexpected results")
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

	var rootfsArgs []string
	if p.rootfs, rootfsArgs, p.env, err = utils.BuildRootfs(ctx, opts.Workdir, opts.Rootfs, opts.Compress, opts.KeepFileOwners, p.architecture.String(), opts.RootfsType); err != nil {
		return nil, fmt.Errorf("could not build rootfs: %w", err)
	}

	if p.env != nil {
		p.env = append(opts.Env, p.env...)
	} else {
		opts.Env = opts.Env
	}

	// If no arguments have been specified, use the ones which are default and
	// that have been included in the package.
	if len(opts.Args) == 0 {
		if opts.Project != nil && len(opts.Project.Command()) > 0 {
			p.args = opts.Project.Command()
		} else if rootfsArgs != nil {
			p.args = rootfsArgs
		} else if p.target != nil && len(p.target.Command()) > 0 {
			p.args = p.target.Command()
		}
	}

	var rawRoms []string
	if opts.Project != nil {
		rawRoms = opts.Project.Roms()
	} else if len(opts.Roms) > 0 {
		rawRoms = opts.Roms
	} else if p.target != nil && len(p.target.Roms()) > 0 {
		rawRoms = p.target.Roms()
	}

	// Build ROMs with the specified filesystem type (if provided)
	if p.roms, err = utils.BuildRoms(ctx, opts.Workdir, rawRoms, opts.Compress, opts.KeepFileOwners, p.architecture.String(), opts.RootfsType); err != nil {
		return nil, fmt.Errorf("could not build ROMs: %w", err)
	}

	var labels map[string]string
	if opts.Project != nil {
		labels = opts.Project.Labels()
	}
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
			p.platform.Name()+"/"+p.architecture.Name(),
			func(ctx context.Context) error {
				popts := append(opts.packopts,
					packmanager.PackArgs(p.args...),
					packmanager.PackArchitecture(p.architecture),
					packmanager.PackPlatform(p.platform),
					packmanager.PackName(opts.Name),
					packmanager.PackOutput(opts.Output),
					packmanager.PackLabels(labels),
				)

				if len(p.kernel) > 0 {
					popts = append(popts,
						packmanager.PackKernel(p.kernel),
					)
				}

				if p.rootfs != nil {
					popts = append(popts,
						packmanager.PackInitrd(p.rootfs),
					)
				}

				if len(p.roms) > 0 {
					popts = append(popts,
						packmanager.PackRoms(p.roms...),
					)
				}

				if !opts.NoKConfig && p.kconfig != nil {
					popts = append(popts,
						packmanager.PackKConfig(p.kconfig),
					)

					if ukversion, ok := p.kconfig.Get(unikraft.UK_FULLVERSION); ok {
						popts = append(popts,
							packmanager.PackWithKernelVersion(ukversion.Value),
						)
					}
				}

				envs := opts.aggregateEnvs()
				if len(envs) > 0 {
					popts = append(popts, packmanager.PackWithEnvs(envs))
				} else if len(opts.Env) > 0 {
					popts = append(popts, packmanager.PackWithEnvs(opts.Env))
				}

				more, err := opts.pm.Pack(ctx, p.target, popts...)
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

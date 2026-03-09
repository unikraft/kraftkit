// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-shellwords"
	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

// initProject sets up the project based on the provided context and
// options.
func (opts *GithubAction) initProject(ctx context.Context) error {
	var err error

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.Workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Interpret the project directory
	opts.project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return err
	}

	return nil
}

// RewrapAsKraftCloudPackage returns the equivalent package name as a
// KraftCloud package.
func (opts *GithubAction) rewrapAsKraftCloudPackage(name string) string {
	name = strings.Replace(name, "unikraft.org/", "index.unikraft.io/", 1)

	if strings.HasPrefix(name, "unikraft.io") {
		name = "index." + name
	} else if strings.Contains(name, "/") && !strings.Contains(name, "unikraft.io") {
		name = "index.unikraft.io/" + name
	} else if !strings.HasPrefix(name, "index.unikraft.io") {
		name = "index.unikraft.io/official/" + name
	}

	return name
}

// aggregateEnvs aggregates the environment variables from the project and
// the cli options, filling in missing values with the host environment.
func (opts *GithubAction) aggregateEnvs() []string {
	envs := make(map[string]string)

	if opts.project != nil && opts.project.Env() != nil {
		envs = opts.project.Env()
	}

	// Aggregate all the environment variables
	var env []string
	for k, v := range envs {
		env = append(env, k+"="+v)
	}

	return env
}

// BuildRootfs generates a rootfs based on the provided working directory and
// the rootfs entrypoint for the provided target(s).
func (opts *GithubAction) buildRootfs(ctx context.Context, workdir, rootfs string, compress bool, arch string, fsType initrd.FsType) (initrd.Initrd, []string, []string, error) {
	if rootfs == "" || fsType == "" {
		return nil, nil, nil, nil
	}

	var cmds []string
	var envs []string

	ramfs, err := initrd.New(ctx,
		rootfs,
		initrd.WithWorkdir(workdir),
		initrd.WithOutput(filepath.Join(
			workdir,
			unikraft.BuildDir,
			fmt.Sprintf(initrd.DefaultInitramfsArchFileName, arch, fsType),
		)),
		initrd.WithCacheDir(filepath.Join(
			workdir,
			unikraft.VendorDir,
			"rootfs-cache",
		)),
		initrd.WithArchitecture(arch),
		initrd.WithOutputType(fsType),
		initrd.WithCompression(compress),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not initialize initramfs builder: %w", err)
	}

	rootfs, err = ramfs.Build(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	// Always overwrite the existing cmds and envs, considering this will
	// be the same regardless of the target.
	cmds = ramfs.Args()
	envs = ramfs.Env()

	return ramfs, cmds, envs, nil
}

func (opts *GithubAction) packagableUnikraft(ctx context.Context) (bool, error) {
	if opts.project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.project.Unikraft(ctx) == nil {
		return false, fmt.Errorf("cannot package without unikraft core specification")
	}

	if opts.project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.project.Rootfs()
	}

	if opts.project.InitrdFsType().String() != "" && opts.RootfsType == "" {
		opts.RootfsType = opts.project.InitrdFsType().String()
	}

	return true, nil
}

func (opts *GithubAction) packagableRuntime(ctx context.Context) (bool, error) {
	if opts.project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.project.Runtime() == nil {
		return false, fmt.Errorf("cannot package without unikraft core specification")
	}

	if opts.project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.project.Rootfs()
	}

	if opts.project.InitrdFsType().String() != "" && opts.RootfsType == "" {
		opts.RootfsType = opts.project.InitrdFsType().String()
	}

	return true, nil
}

func (opts *GithubAction) packagableDockerfile(ctx context.Context) (bool, error) {
	if opts.project == nil {
		// Do not capture the the project is not initialized, as we can still build
		// the unikernel using the Dockerfile provided with the `--rootfs`.
		_ = opts.initProject(ctx)
	}

	if opts.project != nil && opts.project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.project.Rootfs()
	}

	if opts.project != nil && opts.project.InitrdFsType().String() != "" && opts.RootfsType == "" {
		opts.RootfsType = opts.project.InitrdFsType().String()
	}

	// TODO(nderjung): This is a very naiive check and should be improved,
	// potentially using an external library which parses the Dockerfile syntax.
	// In most cases, however, the Dockerfile is usually named `Dockerfile`.
	if !strings.Contains(strings.ToLower(opts.Rootfs), "dockerfile") {
		return false, fmt.Errorf("%s is not a Dockerfile", opts.Rootfs)
	}

	return true, nil
}

func (opts *GithubAction) packUnikraft(ctx context.Context, output string, format pack.PackageFormat) error {
	var err error
	pm := packmanager.G(ctx)

	// Switch the package manager the desired format for this target
	if format != "auto" {
		pm, err = pm.From(format)
		if err != nil {
			return err
		}
	}

	var cmdShellArgs []string

	// If no arguments have been specified, use the ones which are default and
	// that have been included in the package.
	if len(opts.Args) == 0 {
		if len(opts.project.Command()) > 0 {
			cmdShellArgs = opts.project.Command()
		} else if len(opts.target.Command()) > 0 {
			cmdShellArgs = opts.target.Command()
		}

		cmdShellArgs, err = shellwords.Parse(strings.Join(cmdShellArgs, " "))
		if err != nil {
			return err
		}
	} else {
		cmdShellArgs = strings.Split(opts.Args, " ")
	}

	popts := []packmanager.PackOption{
		packmanager.PackInitrd(opts.target.Initrd()),
		packmanager.PackKConfig(opts.target.KConfig()),
		packmanager.PackName(output),
		packmanager.PackMergeStrategy(packmanager.MergeStrategy(opts.Strategy)),
		packmanager.PackArgs(cmdShellArgs...),
	}

	if ukversion, ok := opts.target.KConfig().Get(unikraft.UK_FULLVERSION); ok {
		popts = append(popts,
			packmanager.PackWithKernelVersion(ukversion.Value),
		)
	}

	packs, err := pm.Pack(ctx, opts.target, popts...)
	if err != nil {
		return err
	}

	if opts.Push {
		return packs[0].Push(ctx)
	}

	return nil
}

func (opts *GithubAction) packRuntime(ctx context.Context, output string, format pack.PackageFormat) error {
	var err error
	var targ target.Target
	var runtimeName string

	if opts.project == nil || opts.project.Runtime() == nil {
		return fmt.Errorf("cannot use runtime packager without a project runtime")
	}
	runtimeName = opts.project.Runtime().Name()

	if opts.Plat == "kraftcloud" || (opts.project.Runtime().Platform() != nil && opts.project.Runtime().Platform().Name() == "kraftcloud") {
		runtimeName = opts.rewrapAsKraftCloudPackage(runtimeName)
	}

	targets := opts.project.Targets()
	qopts := []packmanager.QueryOption{
		packmanager.WithName(runtimeName),
		packmanager.WithVersion(opts.project.Runtime().Version()),
	}

	if len(targets) == 1 {
		targ = targets[0]
	} else if len(targets) > 1 {
		// Filter project targets by any provided CLI options
		targets = target.Filter(
			targets,
			opts.Arch,
			opts.Plat,
			opts.Target,
		)

		switch {
		case len(targets) == 0:
			return fmt.Errorf("could not detect any project targets based on plat=\"%s\" arch=\"%s\"", opts.Plat, opts.Arch)

		case len(targets) == 1:
			targ = targets[0]

		case config.G[config.KraftKit](ctx).NoPrompt && len(targets) > 1:
			return fmt.Errorf("could not determine what to run based on provided CLI arguments")

		default:
			targ, err = target.Select(targets)
			if err != nil {
				return fmt.Errorf("could not select target: %v", err)
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

		if opts.Plat == "" {
			opts.Plat = targ.Platform().Name()
		}
		if opts.Arch == "" {
			opts.Arch = targ.Architecture().Name()
		}
	}

	pm := packmanager.G(ctx)

	// Switch the package manager the desired format for this target
	if format != "auto" {
		pm, err = pm.From(format)
		if err != nil {
			return err
		}
	}

	qopts = append(qopts,
		packmanager.WithArchitecture(opts.Arch),
		packmanager.WithPlatform(opts.Plat),
		packmanager.WithKConfig(kconfigs),
	)

	packs, err = pm.Catalog(ctx, append(qopts, packmanager.WithRemote(false))...)
	if err != nil {
		return fmt.Errorf("could not query catalog: %w", err)
	} else if len(packs) == 0 {
		// Try again with a remote update request.  Save this to qopts in case we
		// need to call `Catalog` again.
		packs, err = pm.Catalog(ctx, append(qopts, packmanager.WithRemote(true))...)
		if err != nil {
			return fmt.Errorf("could not query catalog: %w", err)
		}
	}

	if len(packs) == 0 {
		if len(opts.Plat) > 0 && len(opts.Arch) > 0 {
			return fmt.Errorf(
				"could not find runtime '%s:%s' (%s/%s)",
				opts.project.Runtime().Name(),
				opts.project.Runtime().Version(),
				opts.Plat,
				opts.Arch,
			)
		} else if len(opts.Arch) > 0 {
			return fmt.Errorf(
				"could not find runtime '%s:%s' with '%s' architecture",
				opts.project.Runtime().Name(),
				opts.project.Runtime().Version(),
				opts.Arch,
			)
		} else if len(opts.Plat) > 0 {
			return fmt.Errorf(
				"could not find runtime '%s:%s' with '%s' platform",
				opts.project.Runtime().Name(),
				opts.project.Runtime().Version(),
				opts.Plat,
			)
		} else {
			return fmt.Errorf(
				"could not find runtime %s:%s",
				opts.project.Runtime().Name(),
				opts.project.Runtime().Version(),
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
				if pt.Architecture().String() == opts.Arch && pt.Platform().String() == opts.Plat {
					found = append(found, p)
				}
			}

			// Could not find a package that matches the desired architecture and
			// platform, prompt with available set of packages.
			if len(found) == 0 {
				if !config.G[config.KraftKit](ctx).NoPrompt {
					log.G(ctx).Warnf("could not find package '%s:%s' based on %s/%s", runtimeName, opts.project.Runtime().Version(), opts.Plat, opts.Arch)
					p, err := selection.Select[pack.Package]("select alternative package with same name to continue", packs...)
					if err != nil {
						return fmt.Errorf("could not select package: %w", err)
					}

					selected = p
				} else {
					return fmt.Errorf("could not find package '%s:%s' based on %s/%s but %d others found but prompting has been disabled", runtimeName, opts.project.Runtime().Version(), opts.Plat, opts.Arch, len(packs))
				}
			} else if len(found) == 1 {
				selected = &found[0]
			} else { // > 1
				if !config.G[config.KraftKit](ctx).NoPrompt {
					log.G(ctx).Infof("found %d packages named '%s:%s' based on %s/%s", len(found), runtimeName, opts.project.Runtime().Version(), opts.Plat, opts.Arch)
					p, err := selection.Select[pack.Package]("select package to continue", found...)
					if err != nil {
						return fmt.Errorf("could not select package: %w", err)
					}

					selected = p
				} else {
					return fmt.Errorf("found %d packages named '%s:%s' based on %s/%s but prompting has been disabled", len(found), runtimeName, opts.project.Runtime().Version(), opts.Plat, opts.Arch)
				}
			}
		} else {
			selected, err = selection.Select[pack.Package]("multiple runtimes available", packs...)
			if err != nil {
				return err
			}
		}
	}

	runtime := *selected
	pulled, _, _ := runtime.PulledAt(ctx)

	// Temporarily save the runtime package.
	if err := runtime.Save(ctx); err != nil {
		return fmt.Errorf("could not save runtime package: %w", err)
	}

	// Remove the cached runtime package reference if it was not previously
	// pulled.
	if !pulled {
		defer func() {
			if err := runtime.Delete(ctx); err != nil {
				log.G(ctx).Debugf("could not delete intermediate runtime package: %s", err.Error())
			}
		}()
	}

	// Create a temporary directory we can use to store the artifacts from
	// pulling and extracting the identified package.
	tempDir, err := os.MkdirTemp("", "kraft-pkg-")
	if err != nil {
		return fmt.Errorf("could not create temporary directory: %w", err)
	}

	defer func() {
		os.RemoveAll(tempDir)
	}()

	// Crucially, the catalog should return an interface that also implements
	// target.Target.  This demonstrates that the implementing package can
	// resolve application kernels.
	targ, ok := runtime.(target.Target)
	if !ok {
		return fmt.Errorf("package does not convert to target")
	}

	var cmds []string
	var rootfsEnvs []string
	var rootfs initrd.Initrd
	if rootfs, cmds, rootfsEnvs, err = opts.buildRootfs(ctx, opts.Workdir, opts.Rootfs, false, targ.Architecture().String(), initrd.FsType(opts.RootfsType)); err != nil {
		return fmt.Errorf("could not build rootfs: %w", err)
	}

	args := []string{}

	// If no arguments have been specified, use the ones which are default and
	// that have been included in the package.
	if len(opts.Args) == 0 {
		if len(opts.project.Command()) > 0 {
			args = opts.project.Command()
		} else if cmds != nil {
			args = cmds
		} else if len(targ.Command()) > 0 {
			args = targ.Command()
		}

		args, err = shellwords.Parse(fmt.Sprintf("'%s'", strings.Join(args, "' '")))
		if err != nil {
			return err
		}
	} else {
		args = strings.Split(opts.Args, " ")
	}

	labels := opts.project.Labels()

	var popts []packmanager.PackOption
	popts = append(popts,
		packmanager.PackArgs(args...),
		packmanager.PackInitrd(rootfs),
		packmanager.PackKConfig(targ.KConfig()),
		packmanager.PackName(output),
		packmanager.PackOutput(output),
		packmanager.PackLabels(labels),
		packmanager.PackMergeStrategy(packmanager.MergeStrategy(opts.Strategy)),
	)

	if ukversion, ok := targ.KConfig().Get(unikraft.UK_FULLVERSION); ok {
		popts = append(popts,
			packmanager.PackWithKernelVersion(ukversion.Value),
		)
	}

	envs := opts.aggregateEnvs()
	if len(envs) > 0 {
		popts = append(popts, packmanager.PackWithEnvs(envs))
	} else if len(rootfsEnvs) > 0 {
		popts = append(popts, packmanager.PackWithEnvs(rootfsEnvs))
	}

	packaged, err := pm.Pack(ctx, targ, popts...)
	if err != nil {
		return err
	}

	if opts.Push {
		return packaged[0].Push(ctx)
	}

	return nil
}

func (opts *GithubAction) packDockerfile(ctx context.Context, output string, format pack.PackageFormat) error {
	return opts.packRuntime(ctx, output, format)
}

// pack
func (opts *GithubAction) packAndPush(ctx context.Context) error {
	output := opts.Output
	var format pack.PackageFormat
	if strings.Contains(opts.Output, "://") {
		split := strings.SplitN(opts.Output, "://", 2)
		format = pack.PackageFormat(split[0])
		output = split[1]
	} else {
		format = "oci"
	}

	// Purge the package manager before we start packaging such that we can ensure
	// that we are not packaging any stale data.
	if err := packmanager.G(ctx).Purge(ctx); err != nil {
		return fmt.Errorf("package manager could not clean: %w", err)
	}

	if packagable, err := opts.packagableUnikraft(ctx); packagable && err == nil {
		err := opts.packUnikraft(ctx, output, format)
		if err != nil {
			return err
		}
	} else if packagable, err := opts.packagableRuntime(ctx); packagable && err == nil {
		err := opts.packRuntime(ctx, output, format)
		if err != nil {
			return err
		}
	} else if packagable, err := opts.packagableDockerfile(ctx); packagable && err == nil {
		err := opts.packDockerfile(ctx, output, format)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("no suitable packager found")
	}

	return nil
}

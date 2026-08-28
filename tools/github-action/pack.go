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
	"kraftkit.sh/internal/cli/kraft/utils"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	ukruntime "kraftkit.sh/unikraft/runtime"
	"kraftkit.sh/unikraft/target"
	kraftfilev07 "unikraft.com/x/kraftfile"
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

// formatRuntimeReference renders a runtime name and version as a single
// reference, leaving names which already carry an explicit tag untouched.
func formatRuntimeReference(name, version string) string {
	if version == "" || ukruntime.HasExplicitTag(name) {
		return name
	}

	return fmt.Sprintf("%s:%s", name, version)
}

// normalizeRuntimeNameVersion defaults the version to `latest` unless the name
// already carries an explicit tag.
func normalizeRuntimeNameVersion(name, version string) (string, string) {
	if version == "" && !ukruntime.HasExplicitTag(name) {
		version = "latest"
	}

	return name, version
}

// resolveRuntimeNameVersion determines which runtime to package against, giving
// precedence to the `runtime` input over the one declared in the Kraftfile.
func resolveRuntimeNameVersion(runtimeFlag string, projectRuntime *ukruntime.Runtime) (string, string) {
	switch {
	case len(runtimeFlag) > 0:
		runtime := &ukruntime.Runtime{}
		runtime.SetName(runtimeFlag)
		return normalizeRuntimeNameVersion(runtime.Name(), runtime.Version())
	case projectRuntime != nil:
		return normalizeRuntimeNameVersion(projectRuntime.Name(), projectRuntime.Version())
	default:
		return "", ""
	}
}

// aggregateEnvs aggregates the environment variables from the project and
// the action inputs, filling in missing values with the host environment.
func (opts *GithubAction) aggregateEnvs(penvs []string) []string {
	envs := make(map[string]string)

	// Add the packager environment (Dockerfile, etc).
	for _, env := range penvs {
		if strings.ContainsRune(env, '=') {
			parts := strings.SplitN(env, "=", 2)
			envs[parts[0]] = parts[1]
			continue
		}

		envs[env] = os.Getenv(env)
	}

	// Add the project environment.
	if opts.project != nil && opts.project.Env() != nil {
		for k, v := range opts.project.Env() {
			envs[k] = v
		}
	}

	// Add the environment supplied via the action inputs.
	for _, env := range opts.envs {
		if strings.ContainsRune(env, '=') {
			parts := strings.SplitN(env, "=", 2)
			envs[parts[0]] = parts[1]
			continue
		}

		envs[env] = os.Getenv(env)
	}

	// Aggregate all the environment variables.
	var env []string
	for k, v := range envs {
		env = append(env, k+"="+v)
	}

	return env
}

// aggregateLabels merges the labels declared in the Kraftfile with those
// supplied via the action inputs.
func (opts *GithubAction) aggregateLabels() (map[string]string, error) {
	labels := map[string]string{}
	if opts.project != nil {
		for k, v := range opts.project.Labels() {
			labels[k] = v
		}
	}

	for _, label := range opts.labels {
		k, v, ok := strings.Cut(label, "=")
		if !ok {
			return nil, fmt.Errorf("invalid label format: %s", label)
		}

		labels[k] = v
	}

	return labels, nil
}

// buildRoms builds the auxiliary ROM filesystems declared either in the
// Kraftfile, via the `roms` input or by the resolved target.
func (opts *GithubAction) buildRoms(ctx context.Context, targ target.Target, arch string) ([]kraftfilev07.FS, error) {
	var raw []kraftfilev07.FS

	switch {
	case opts.project != nil && len(opts.project.Roms()) > 0:
		raw = opts.project.Roms()
	case len(opts.roms) > 0:
		for _, rom := range opts.roms {
			raw = append(raw, kraftfilev07.FS{
				Source: &kraftfilev07.FSSource{Path: rom},
			})
		}
	case targ != nil && len(targ.Roms()) > 0:
		raw = targ.Roms()
	}

	if len(raw) == 0 {
		return nil, nil
	}

	roms, err := initrd.BuildRoms(
		ctx,
		opts.Workdir,
		raw,
		opts.Compress,
		opts.KeepFileOwners,
		arch,
		kraftfilev07.FsType(opts.RomType),
	)
	if err != nil {
		return nil, fmt.Errorf("could not build ROMs: %w", err)
	}

	return roms, nil
}

// kconfigPackOptions returns the KConfig metadata to include in the package,
// applying any overrides supplied via the action inputs.
func (opts *GithubAction) kconfigPackOptions(kconfigs kconfig.KeyValueMap) ([]packmanager.PackOption, error) {
	if kconfigs == nil {
		return nil, nil
	}

	var popts []packmanager.PackOption

	// The Unikraft version is recorded even when the KConfig metadata itself is
	// not included in the package.
	if ukversion, ok := kconfigs.Get(unikraft.UK_FULLVERSION); ok {
		popts = append(popts, packmanager.PackWithKernelVersion(ukversion.Value))
	}

	if opts.NoKConfig {
		return popts, nil
	}

	if len(opts.KConfigFile) > 0 {
		kv, err := kconfig.NewKeyValueMapFromFile(opts.KConfigFile)
		if err != nil {
			return nil, fmt.Errorf("could not read KConfig file: %w", err)
		}

		kconfigs.OverrideBy(kv)
	}

	if len(opts.setKConfig) > 0 {
		kv, err := kconfig.NewKeyValueMapFromSlice(opts.setKConfig)
		if err != nil {
			return nil, fmt.Errorf("could not read set_kconfig: %w", err)
		}

		kconfigs.OverrideBy(kv)
	}

	return append(popts, packmanager.PackKConfig(kconfigs)), nil
}

// BuildRootfs generates a rootfs based on the provided working directory and
// the rootfs entrypoint for the provided target(s).
func (opts *GithubAction) buildRootfs(ctx context.Context, workdir, rootfs string, arch string, fsType kraftfilev07.FsType) (initrd.Initrd, []string, []string, error) {
	if rootfs == "" {
		return nil, nil, nil, nil
	}

	// Neither the inputs nor the Kraftfile declared a format for the root file
	// system, fall back to the default.
	if fsType == "" {
		fsType = initrd.FsTypeCpio
	}

	return initrd.BuildRootfs(
		ctx,
		initrd.WithRootfsPath(rootfs),
		initrd.WithWorkdir(workdir),
		initrd.WithOutput(filepath.Join(
			opts.buildOutputDir(),
			fmt.Sprintf(initrd.DefaultInitramfsArchFileName, arch, fsType),
		)),
		initrd.WithCacheDir(filepath.Join(
			workdir,
			unikraft.VendorDir,
			"rootfs-cache",
		)),
		initrd.WithArchitecture(arch),
		initrd.WithOutputType(fsType),
		initrd.WithKeepOwners(opts.KeepFileOwners),
		initrd.WithCompression(opts.Compress),
	)
}

// inheritProjectRootfs adopts the root filesystem declared by the project,
// unless one was already supplied via the action inputs or the root filesystem
// has been disabled altogether.
func (opts *GithubAction) inheritProjectRootfs() {
	if opts.NoRootfs {
		opts.Rootfs = ""
		return
	}

	if opts.project == nil {
		return
	}

	if opts.project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.project.Rootfs()
	}

	if opts.project.InitrdFsType().String() != "" && opts.RootfsType == "" {
		opts.RootfsType = opts.project.InitrdFsType().String()
	}
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

	opts.inheritProjectRootfs()

	return true, nil
}

func (opts *GithubAction) packagableRuntime(ctx context.Context) (bool, error) {
	if opts.project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.project.Runtime() == nil && opts.Runtime == "" && len(opts.project.Rootfs()) == 0 && len(opts.project.Roms()) == 0 {
		return false, fmt.Errorf("cannot package without any of runtime, rootfs or roms")
	}

	opts.inheritProjectRootfs()

	return true, nil
}

func (opts *GithubAction) packagableDockerfile(ctx context.Context) (bool, error) {
	if opts.project == nil {
		// Do not capture the the project is not initialized, as we can still build
		// the unikernel using the Dockerfile provided with the `--rootfs`.
		_ = opts.initProject(ctx)
	}

	opts.inheritProjectRootfs()

	// TODO(nderjung): This is a very naiive check and should be improved,
	// potentially using an external library which parses the Dockerfile syntax.
	// In most cases, however, the Dockerfile is usually named `Dockerfile`.
	if !strings.Contains(strings.ToLower(opts.Rootfs), "dockerfile") {
		return false, fmt.Errorf("%s is not a Dockerfile", opts.Rootfs)
	}

	return true, nil
}

func (opts *GithubAction) packUnikraft(ctx context.Context, name, output string, format pack.PackageFormat) error {
	var err error
	pm := packmanager.G(ctx)

	// Switch the package manager the desired format for this target.
	if format != "auto" {
		pm, err = pm.From(format)
		if err != nil {
			return err
		}
	}

	targ := opts.target

	if !filepath.IsAbs(targ.Kernel()) {
		targ.SetKernelPath(filepath.Join(opts.Workdir, targ.Kernel()))
	}

	var cmds []string
	var penvs []string
	var rootfs initrd.Initrd

	// Build the rootfs only when it is not already embedded in the kernel.
	if opts.project.KConfig().AllNoOrUnset(
		"CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD",
		"CONFIG_LIBVFSCORE_AUTOMOUNT_CI_EINITRD",
		"CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD_PATH",
		"CONFIG_LIBPOSIX_VFS_FSTAB_EINITRD",
		"CONFIG_LIBPOSIX_VFS_FSTAB_EINITRD_PATH",
		"CONFIG_LIBPOSIX_VFS_FSTAB_BUILTIN_EINITRD",
		"CONFIG_LIBPOSIX_VFS_FSTAB_FALLBACK_EINITRD",
	) {
		rootfs, cmds, penvs, err = opts.buildRootfs(
			ctx,
			opts.Workdir,
			opts.Rootfs,
			targ.Architecture().String(),
			kraftfilev07.FsType(opts.RootfsType),
		)
		if err != nil {
			return fmt.Errorf("could not build rootfs: %w", err)
		}
	}

	roms, err := opts.buildRoms(ctx, targ, targ.Architecture().String())
	if err != nil {
		return err
	}

	cmdArgs := []string{}

	// If no arguments have been specified, use the ones which are default and
	// that have been included in the package.
	if opts.Args == "" {
		if len(opts.project.Command()) > 0 {
			cmdArgs = opts.project.Command()
		} else if len(targ.Command()) > 0 {
			cmdArgs = targ.Command()
		} else if cmds != nil {
			cmdArgs = cmds
		}
	} else {
		cmdArgs = strings.Split(opts.Args, " ")
	}

	cmdShellArgs, err := shellwords.Parse(strings.Join(cmdArgs, " "))
	if err != nil {
		return err
	}

	labels, err := opts.aggregateLabels()
	if err != nil {
		return err
	}

	popts := []packmanager.PackOption{
		packmanager.PackArchitecture(targ.Architecture()),
		packmanager.PackPlatform(targ.Platform()),
		packmanager.PackArgs(cmdShellArgs...),
		packmanager.PackInitrd(rootfs),
		packmanager.PackName(name),
		packmanager.PackOutput(output),
		packmanager.PackLabels(labels),
		packmanager.PackMergeStrategy(packmanager.MergeStrategy(opts.Strategy)),
	}

	if len(roms) > 0 {
		popts = append(popts, packmanager.PackRoms(roms...))
	}

	kopts, err := opts.kconfigPackOptions(targ.KConfig())
	if err != nil {
		return err
	}

	popts = append(popts, kopts...)

	envs := opts.aggregateEnvs(penvs)
	if len(envs) > 0 {
		popts = append(popts, packmanager.PackWithEnvs(envs))
	}

	packs, err := pm.Pack(ctx, targ, popts...)
	if err != nil {
		return err
	}

	if opts.Push {
		if len(packs) == 0 {
			return nil
		}

		return packs[0].Push(ctx,
			pack.WithPushAuthConfig(config.G[config.KraftKit](ctx).Auth),
		)
	}

	return nil
}

func (opts *GithubAction) packRuntime(ctx context.Context, name, output string, format pack.PackageFormat) error {
	var err error
	var targ target.Target

	if opts.project == nil {
		return fmt.Errorf("cannot use runtime packager without a project")
	}

	projectRuntime := opts.project.Runtime()

	runtimeName, runtimeVersion := resolveRuntimeNameVersion(opts.Runtime, projectRuntime)
	if runtimeName == "" {
		return fmt.Errorf("cannot use runtime packager without a runtime")
	}

	if opts.Plat == "kraftcloud" || (projectRuntime != nil && projectRuntime.Platform() != nil && projectRuntime.Platform().Name() == "kraftcloud") {
		runtimeName = utils.RewrapAsKraftCloudPackage(runtimeName)
	}

	runtimeRef := formatRuntimeReference(runtimeName, runtimeVersion)

	targets := opts.project.Targets()
	qopts := []packmanager.QueryOption{
		packmanager.WithName(runtimeName),
		packmanager.WithVersion(runtimeVersion),
	}

	if len(targets) == 1 {
		targ = targets[0]
	} else if len(targets) > 1 {
		// Filter project targets by any provided CLI options.
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

	// Switch the package manager the desired format for this target.
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
		// Try again with a remote update request.
		packs, err = pm.Catalog(ctx, append(qopts, packmanager.WithRemote(true))...)
		if err != nil {
			return fmt.Errorf("could not query catalog: %w", err)
		}
	}

	if len(packs) == 0 {
		if len(opts.Plat) > 0 && len(opts.Arch) > 0 {
			return fmt.Errorf(
				"could not find runtime '%s' (%s/%s)",
				runtimeRef,
				opts.Plat,
				opts.Arch,
			)
		} else if len(opts.Arch) > 0 {
			return fmt.Errorf(
				"could not find runtime '%s' with '%s' architecture",
				runtimeRef,
				opts.Arch,
			)
		} else if len(opts.Plat) > 0 {
			return fmt.Errorf(
				"could not find runtime '%s' with '%s' platform",
				runtimeRef,
				opts.Plat,
			)
		} else {
			return fmt.Errorf(
				"could not find runtime %s",
				runtimeRef,
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

			if len(found) == 0 {
				if !config.G[config.KraftKit](ctx).NoPrompt {
					log.G(ctx).Warnf("could not find package '%s' based on %s/%s", runtimeRef, opts.Plat, opts.Arch)
					p, err := selection.Select("select alternative package with same name to continue", packs...)
					if err != nil {
						return fmt.Errorf("could not select package: %w", err)
					}

					selected = p
				} else {
					return fmt.Errorf("could not find package '%s' based on %s/%s but %d others found but prompting has been disabled", runtimeRef, opts.Plat, opts.Arch, len(packs))
				}
			} else if len(found) == 1 {
				selected = &found[0]
			} else { // > 1
				if !config.G[config.KraftKit](ctx).NoPrompt {
					log.G(ctx).Infof("found %d packages named '%s' based on %s/%s", len(found), runtimeRef, opts.Plat, opts.Arch)
					p, err := selection.Select("select package to continue", found...)
					if err != nil {
						return fmt.Errorf("could not select package: %w", err)
					}

					selected = p
				} else {
					return fmt.Errorf("found %d packages named '%s' based on %s/%s but prompting has been disabled", len(found), runtimeRef, opts.Plat, opts.Arch)
				}
			}
		} else {
			if !config.G[config.KraftKit](ctx).NoPrompt {
				selected, err = selection.Select("multiple runtimes available", packs...)
				if err != nil {
					return err
				}
			} else {
				return fmt.Errorf("multiple runtimes available for '%s' but prompting has been disabled", runtimeRef)
			}
		}
	}

	runtime := *selected
	pulled, _, _ := runtime.PulledAt(ctx)

	// Temporarily save the runtime package.
	if err := runtime.Save(ctx); err != nil {
		return fmt.Errorf("could not save runtime package: %w", err)
	}

	if !pulled {
		if err := runtime.Pull(
			ctx,
			pack.WithPullWorkdir(opts.Workdir),
			pack.WithPullAuthConfig(config.G[config.KraftKit](ctx).Auth),
		); err != nil {
			return fmt.Errorf("could not pull runtime package: %w", err)
		}
	}

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
	if rootfs, cmds, rootfsEnvs, err = opts.buildRootfs(
		ctx,
		opts.Workdir,
		opts.Rootfs,
		targ.Architecture().String(),
		kraftfilev07.FsType(opts.RootfsType),
	); err != nil {
		return fmt.Errorf("could not build rootfs: %w", err)
	}

	roms, err := opts.buildRoms(ctx, targ, targ.Architecture().String())
	if err != nil {
		return err
	}

	args := []string{}

	// If no arguments have been specified, use the ones which are default and
	// that have been included in the package.
	if opts.Args == "" {
		if len(opts.project.Command()) > 0 {
			args = opts.project.Command()
		} else if cmds != nil {
			args = cmds
		} else if len(targ.Command()) > 0 {
			args = targ.Command()
		}
	} else {
		args = strings.Split(opts.Args, " ")
	}

	args, err = shellwords.Parse(strings.Join(args, " "))
	if err != nil {
		return err
	}

	labels, err := opts.aggregateLabels()
	if err != nil {
		return err
	}

	popts := []packmanager.PackOption{
		packmanager.PackArchitecture(targ.Architecture()),
		packmanager.PackPlatform(targ.Platform()),
		packmanager.PackArgs(args...),
		packmanager.PackInitrd(rootfs),
		packmanager.PackName(name),
		packmanager.PackOutput(output),
		packmanager.PackLabels(labels),
		packmanager.PackMergeStrategy(packmanager.MergeStrategy(opts.Strategy)),
	}

	if len(targ.Kernel()) > 0 {
		popts = append(popts, packmanager.PackKernel(targ.Kernel()))
	}

	if len(roms) > 0 {
		popts = append(popts, packmanager.PackRoms(roms...))
	}

	kopts, err := opts.kconfigPackOptions(targ.KConfig())
	if err != nil {
		return err
	}

	popts = append(popts, kopts...)

	envs := opts.aggregateEnvs(rootfsEnvs)
	if len(envs) > 0 {
		popts = append(popts, packmanager.PackWithEnvs(envs))
	}

	packaged, err := pm.Pack(ctx, targ, popts...)
	if err != nil {
		return err
	}

	if opts.Push {
		if len(packaged) == 0 {
			return nil
		}

		return packaged[0].Push(ctx,
			pack.WithPushAuthConfig(config.G[config.KraftKit](ctx).Auth),
		)
	}

	return nil
}

func (opts *GithubAction) packDockerfile(ctx context.Context, name, output string, format pack.PackageFormat) error {
	return opts.packRuntime(ctx, name, output, format)
}

// pack
func (opts *GithubAction) packAndPush(ctx context.Context) error {
	output := opts.Output
	format := pack.PackageFormat("oci")
	if strings.Contains(opts.Output, "://") {
		split := strings.SplitN(opts.Output, "://", 2)
		format = pack.PackageFormat(split[0])
		output = split[1]
	}

	// The package reference is taken from the output when one has been supplied,
	// which preserves the historic behaviour of this action, and otherwise falls
	// back to the `name` input.
	name := output
	if name == "" {
		name = opts.Name
	}

	if name == "" {
		return fmt.Errorf("cannot package without setting either name or output")
	}

	// Purge the package manager before we start packaging such that we can ensure
	// that we are not packaging any stale data.
	if err := packmanager.G(ctx).Purge(ctx); err != nil {
		return fmt.Errorf("package manager could not clean: %w", err)
	}

	if packagable, err := opts.packagableUnikraft(ctx); packagable && err == nil {
		err := opts.packUnikraft(ctx, name, output, format)
		if err != nil {
			return err
		}
	} else if packagable, err := opts.packagableRuntime(ctx); packagable && err == nil {
		err := opts.packRuntime(ctx, name, output, format)
		if err != nil {
			return err
		}
	} else if packagable, err := opts.packagableDockerfile(ctx); packagable && err == nil {
		err := opts.packDockerfile(ctx, name, output, format)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("no suitable packager found")
	}

	return nil
}

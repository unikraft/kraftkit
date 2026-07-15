// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	kraftfilev07 "unikraft.com/x/kraftfile"

	"kraftkit.sh/exec"
	"kraftkit.sh/initrd"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

// buildOutputDir returns the directory in which build artifacts are placed,
// honouring the project's `outdir` when it has been set in the Kraftfile.
func (opts *GithubAction) buildOutputDir() string {
	if opts.project != nil && opts.project.OutDir() != "" {
		return opts.project.OutDir()
	}

	return filepath.Join(opts.Workdir, unikraft.BuildDir)
}

func (opts *GithubAction) build(ctx context.Context) error {
	opts.inheritProjectRootfs()

	if opts.RootfsType == "" {
		opts.RootfsType = initrd.FsTypeCpio.String()
	}

	if opts.Rootfs != "" {
		rootfs, _, _, err := initrd.BuildRootfs(
			ctx,
			initrd.WithRootfsPath(opts.Rootfs),
			initrd.WithWorkdir(opts.Workdir),
			initrd.WithOutput(filepath.Join(
				opts.buildOutputDir(),
				fmt.Sprintf(initrd.DefaultInitramfsArchFileName, opts.target.Architecture().String(), opts.RootfsType),
			)),
			initrd.WithCacheDir(filepath.Join(
				opts.Workdir,
				unikraft.VendorDir,
				"rootfs-cache",
			)),
			initrd.WithArchitecture(opts.target.Architecture().String()),
			initrd.WithOutputType(kraftfilev07.FsType(opts.RootfsType)),
			initrd.WithKeepOwners(opts.KeepFileOwners),
			initrd.WithCompression(opts.Compress),
		)
		if err != nil {
			return fmt.Errorf("could not build rootfs: %w", err)
		}

		outputPath := ""
		if rootfs != nil {
			outputPath = rootfs.Options().Output()
			opts.initrdPath = outputPath
		}

		// Unset the initrd path since this is now embedded in the unikernel.
		if !opts.project.KConfig().AllNoOrUnset(
			"CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD",
			"CONFIG_LIBVFSCORE_AUTOMOUNT_CI_EINITRD",
			"CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD_PATH",
			"CONFIG_LIBPOSIX_VFS_FSTAB_EINITRD",
			"CONFIG_LIBPOSIX_VFS_FSTAB_EINITRD_PATH",
			"CONFIG_LIBPOSIX_VFS_FSTAB_BUILTIN_EINITRD",
			"CONFIG_LIBPOSIX_VFS_FSTAB_FALLBACK_EINITRD",
		) {
			opts.initrdPath = ""
		}

		// Record the built rootfs on the project so later packaging can reuse it.
		opts.project.SetRootfs(opts.initrdPath)
		opts.project.SetInitrdFsType(kraftfilev07.FsType(opts.RootfsType))
	}

	if opts.project.Unikraft(ctx) == nil {
		return nil
	}

	if err := opts.project.Configure(
		ctx,
		opts.target, // Target-specific options
		nil,         // No extra configuration options
		make.WithSilent(true),
		make.WithExecOptions(
			exec.WithStdin(iostreams.G(ctx).In),
			exec.WithStdout(log.G(ctx).Writer()),
			exec.WithStderr(log.G(ctx).WriterLevel(logrus.WarnLevel)),
		),
	); err != nil {
		return fmt.Errorf("could not configure project: %w", err)
	}

	if err := opts.project.Build(
		ctx,
		opts.target, // Target-specific options
		app.WithBuildMakeOptions(
			make.WithMaxJobs(true),
			make.WithExecOptions(
				exec.WithStdout(log.G(ctx).Writer()),
				exec.WithStderr(log.G(ctx).WriterLevel(logrus.WarnLevel)),
			),
			make.WithVars(opts.toolchain),
		),
	); err != nil {
		return err
	}

	if err := opts.relocateKernel(); err != nil {
		return err
	}

	// NOTE(craciunoiuc): This is currently a workaround to remove empty
	// Makefile.uk files generated wrongly by the build system. Until this
	// is fixed we just delete.
	//
	// See: https://github.com/unikraft/unikraft/issues/1456
	makefile := filepath.Join(opts.Workdir, "Makefile.uk")
	if finfo, err := os.Stat(makefile); err == nil && finfo.Size() == 0 {
		if err := os.Remove(makefile); err != nil {
			return fmt.Errorf("removing empty Makefile.uk: %w", err)
		}
	}

	return nil
}

// relocateKernel moves the kernel image from the path Unikraft's build system
// always writes to ([outdir]/[target]_[plat]-[arch]) to the path the target
// actually expects, which differs whenever the Kraftfile sets `output`.
func (opts *GithubAction) relocateKernel() error {
	tc, ok := opts.target.(*target.TargetConfig)
	if !ok {
		return nil
	}

	standardName, err := target.KernelName(*tc)
	if err != nil {
		return nil
	}

	standardPath := filepath.Join(opts.buildOutputDir(), standardName)
	desiredPath := opts.target.Kernel()

	if !filepath.IsAbs(desiredPath) {
		desiredPath = filepath.Join(opts.Workdir, desiredPath)
		opts.target.SetKernelPath(desiredPath)
	}

	if standardPath == desiredPath {
		return nil
	}

	// The build system may already have written directly to the desired path, in
	// which case there is nothing to relocate.
	if _, err := os.Stat(standardPath); err != nil {
		return nil
	}

	if err := moveFile(standardPath, desiredPath); err != nil {
		return fmt.Errorf("moving kernel to %s: %w", desiredPath, err)
	}

	return nil
}

func moveFile(src, dst string) error {
	// Ensure the destination directory exists.
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	// Try rename first (works within same filesystem/disk).
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fallback to copy + remove for cross-device moves.
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("getting source file info: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying file contents: %w", err)
	}

	// Remove the original file after successful copy.
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("removing original file: %w", err)
	}

	return nil
}

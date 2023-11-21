// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/exec"
	"kraftkit.sh/initrd"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
)

func (opts *GithubAction) build(ctx context.Context) error {
	if opts.Rootfs == "" {
		opts.Rootfs = opts.project.Rootfs()
	}

	if opts.Rootfs != "" {
		ramfs, err := initrd.New(ctx,
			filepath.Join(opts.Workdir, opts.Rootfs),
			initrd.WithOutput(filepath.Join(
				opts.Workdir,
				unikraft.BuildDir,
				fmt.Sprintf(initrd.DefaultInitramfsArchFileName, opts.target.Architecture().String()),
			)),
			initrd.WithCacheDir(filepath.Join(
				opts.Workdir,
				unikraft.BuildDir,
				"rootfs-cache",
			)),
			initrd.WithArchitecture(opts.target.Architecture().String()),
		)
		if err != nil {
			return fmt.Errorf("could not prepare initramfs: %w", err)
		}

		opts.initrdPath, err = ramfs.Build(ctx)
		if err != nil {
			return err
		}
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

	return opts.project.Build(
		ctx,
		opts.target, // Target-specific options
		app.WithBuildMakeOptions(
			make.WithMaxJobs(true),
			make.WithExecOptions(
				exec.WithStdout(log.G(ctx).Writer()),
				exec.WithStderr(log.G(ctx).WriterLevel(logrus.WarnLevel)),
			),
		),
	)
}

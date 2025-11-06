// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"context"
	"fmt"
	"path/filepath"

	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft"
)

// BuildRootfs generates a rootfs based on the provided working directory and
// the rootfs entrypoint for the provided target(s).
func BuildRootfs(ctx context.Context, workdir, rootfs string, compress, keepOwners bool, arch string, fsType initrd.FsType) (initrd.Initrd, []string, []string, error) {
	if rootfs == "" || fsType == "" {
		return nil, nil, nil, nil
	}

	var processes []*processtree.ProcessTreeItem
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
		initrd.WithCompression(compress),
		initrd.WithKeepOwners(keepOwners),
		initrd.WithOutputType(fsType),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not initialize initramfs builder: %w", err)
	}

	processes = append(processes,
		processtree.NewProcessTreeItem(
			fmt.Sprintf("building rootfs via %s", ramfs.Name()),
			arch,
			func(ctx context.Context) error {
				rootfs, err = ramfs.Build(ctx)
				if err != nil {
					return err
				}

				// Always overwrite the existing cmds and envs, considering this will
				// be the same regardless of the target.
				cmds = ramfs.Args()
				envs = ramfs.Env()

				return nil
			},
		),
	)

	model, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithRenderer(log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY),
		},
		processes...,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	if err := model.Start(); err != nil {
		return nil, nil, nil, err
	}

	return ramfs, cmds, envs, nil
}

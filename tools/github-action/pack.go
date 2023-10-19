// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"kraftkit.sh/initrd"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
)

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

	var err error
	pm := packmanager.G(ctx)

	// Switch the package manager the desired format for this target
	if format != "auto" {
		pm, err = pm.From(format)
		if err != nil {
			return err
		}
	}

	if opts.Rootfs == "" {
		opts.Rootfs = opts.project.Rootfs()
	}

	if opts.Rootfs != "" {
		ramfs, err := initrd.New(ctx,
			filepath.Join(opts.Workdir, opts.Rootfs),
			initrd.WithOutput(filepath.Join(opts.Workdir, unikraft.BuildDir, initrd.DefaultInitramfsFileName)),
			initrd.WithCacheDir(filepath.Join(opts.Workdir, unikraft.BuildDir, "rootfs-cache")),
		)
		if err != nil {
			return fmt.Errorf("could not prepare initramfs: %w", err)
		}

		opts.initrdPath, err = ramfs.Build(ctx)
		if err != nil {
			return err
		}
	}

	popts := []packmanager.PackOption{
		packmanager.PackInitrd(opts.initrdPath),
		packmanager.PackKConfig(opts.Kconfig),
		packmanager.PackOutput(output),
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

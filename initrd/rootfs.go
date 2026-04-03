// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package initrd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	kraftfilev07 "unikraft.com/x/kraftfile"

	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft"
)

// BuildRootfs generates a rootfs based on the provided working directory and
// the rootfs entrypoint for the provided target(s).
func BuildRootfs(ctx context.Context, opts ...InitrdOption) (Initrd, []string, []string, error) {
	var bopts InitrdOptions
	for _, opt := range opts {
		if err := opt(&bopts); err != nil {
			return nil, nil, nil, fmt.Errorf("could not apply initrd option: %w", err)
		}
	}

	if bopts.RootfsPath() == "" {
		return nil, nil, nil, nil
	}

	var processes []*processtree.ProcessTreeItem
	var cmds []string
	var envs []string

	ramfs, err := New(ctx, bopts.RootfsPath(), opts...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not initialize initramfs builder: %w", err)
	}

	ramfsOpts := ramfs.Options()

	processes = append(processes,
		processtree.NewProcessTreeItem(
			fmt.Sprintf("building rootfs via %s", ramfs.Name()),
			ramfsOpts.Architecture(),
			func(ctx context.Context) error {
				_, err := ramfs.Build(ctx)
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

// BuildRoms generates ROM filesystems based on the provided ROM FS (kraftfilev07.FS).
func BuildRoms(ctx context.Context, workdir string, roms []kraftfilev07.FS, compress, keepOwners bool, arch string, defaultFsType kraftfilev07.FsType) ([]kraftfilev07.FS, error) {
	if len(roms) == 0 {
		return nil, nil
	}

	var processes []*processtree.ProcessTreeItem
	builtRoms := make([]kraftfilev07.FS, len(roms))
	for i, rom := range roms {
		fsType := rom.Format
		if fsType == "" {
			fsType = defaultFsType
		}

		if fsType == "" {
			builtRoms[i] = rom
			continue
		}

		// Check if the ROM is a directory; if it's a file, skip building and use as-is
		romPath := rom.Source
		if !filepath.IsAbs(rom.Source) {
			romPath = filepath.Join(workdir, rom.Source)
		}

		info, err := os.Stat(romPath)
		if err != nil {
			return nil, fmt.Errorf("could not stat ROM path '%s': %w", rom.Source, err)
		}

		// If it's a regular file, don't try to build it as a filesystem
		if !info.IsDir() {
			// File ROMs must be aligned to page size
			const pageSize = 4096
			if info.Size()%pageSize != 0 {
				return nil, fmt.Errorf("ROM file '%s' size (%d bytes) is not aligned to page size (%d bytes)", rom.Source, info.Size(), pageSize)
			}
			builtRoms[i] = rom
			continue
		}

		ramfs, err := New(ctx,
			rom.Source,
			WithWorkdir(workdir),
			WithOutput(filepath.Join(
				workdir,
				unikraft.BuildDir,
				fmt.Sprintf("rom%d-%s.%s", i+1, arch, fsType),
			)),
			WithCacheDir(filepath.Join(
				workdir,
				unikraft.VendorDir,
				"rom-cache",
			)),
			WithArchitecture(arch),
			WithCompression(compress),
			WithKeepOwners(keepOwners),
			WithOutputType(fsType),
		)
		if err != nil {
			return nil, fmt.Errorf("could not initialize ROM builder for '%s': %w", rom.Source, err)
		}

		processes = append(processes,
			processtree.NewProcessTreeItem(
				fmt.Sprintf("building ROM %d via %s", i+1, ramfs.Name()),
				arch,
				func(ctx context.Context) error {
					builtRom, err := ramfs.Build(ctx)
					if err != nil {
						return err
					}

					builtRoms[i] = kraftfilev07.FS{
						Source: builtRom,
						Format: fsType,
					}
					return nil
				},
			),
		)
	}

	// Only run the process tree if there are directories to build
	if len(processes) > 0 {
		model, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(false),
				processtree.WithRenderer(log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY),
			},
			processes...,
		)
		if err != nil {
			return nil, err
		}

		if err := model.Start(); err != nil {
			return nil, err
		}
	}

	return builtRoms, nil
}

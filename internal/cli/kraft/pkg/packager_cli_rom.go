// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package pkg

import (
	"context"
	"fmt"
	"strings"

	kraftfilev07 "unikraft.com/x/kraftfile"

	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

type packagerCliRom struct {
	// Packaging options
	roms         []kraftfilev07.FS
	architecture arch.Architecture
	platform     plat.Platform
}

// String implements fmt.Stringer.
func (p *packagerCliRom) String() string {
	return "cli-rom"
}

// Packagable implements packager.
func (p *packagerCliRom) Packagable(ctx context.Context, opts *PkgOptions, args ...string) (bool, error) {
	// Package as a rom if no kernel or rootfs is provided, but a platform and roms are provided.
	packageable := len(opts.Kernel) == 0 &&
		len(opts.Rootfs) == 0 &&
		opts.Project == nil &&
		len(opts.Platform) > 0 &&
		len(opts.Roms) > 0
	if packageable {
		if len(opts.Architecture) == 0 && strings.Contains(opts.Platform, "/") {
			opts.Platform, opts.Architecture, _ = strings.Cut(opts.Platform, "/")
		}

		if opts.RomType == kraftfilev07.FsType("") {
			opts.RomType = kraftfilev07.FsTypeCpio
		}

		return true, nil
	}

	if len(opts.Roms) > 0 {
		log.G(ctx).Warn("--roms flag set but must be used in conjunction with -m|--arch and/or -p|--plat")
	}

	return false, fmt.Errorf("cannot package without path to -R|--roms, -m|--arch and -p|--plat")
}

// Pack implements packager.
func (p *packagerCliRom) Pack(ctx context.Context, opts *PkgOptions, args ...string) ([]pack.Package, error) {
	var err error

	ac := arch.NewArchitectureFromOptions(
		arch.WithName(opts.Architecture),
	)
	pc := plat.NewPlatformFromOptions(
		plat.WithName(opts.Platform),
	)

	targ := target.NewTargetFromOptions(
		target.WithArchitecture(ac),
		target.WithPlatform(pc),
		target.WithRoms(opts.Roms),
	)
	if p.roms, err = initrd.BuildRoms(ctx,
		opts.Workdir,
		targ.Roms(),
		opts.Compress,
		opts.KeepFileOwners,
		targ.Architecture().String(),
		opts.RomType,
	); err != nil {
		return nil, fmt.Errorf("could not build ROMs: %w", err)
	}

	labels := make(map[string]string)
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
			"packaging "+opts.Name+" ("+opts.Format+")",
			opts.Platform+"/"+opts.Architecture,
			func(ctx context.Context) error {
				popts := append(opts.packopts,
					packmanager.PackArchitecture(targ.Architecture()),
					packmanager.PackPlatform(targ.Platform()),
					packmanager.PackName(opts.Name),
					packmanager.PackOutput(opts.Output),
					packmanager.PackLabels(labels),
					packmanager.PackRoms(p.roms...),
				)

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

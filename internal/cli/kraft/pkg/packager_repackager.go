// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package pkg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-shellwords"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft/app"
)

type packagerRepackager struct{}

// String implements fmt.Stringer.
func (p *packagerRepackager) String() string {
	return "repackager"
}

// Packagable implements packager.
func (p *packagerRepackager) Packagable(ctx context.Context, opts *PkgOptions, args ...string) (bool, error) {
	if len(opts.Rootfs) > 0 {
		return true, nil
	}

	return false, fmt.Errorf("cannot repackage without path to --rootfs")
}

// Pack implements packager.
func (p *packagerRepackager) Pack(ctx context.Context, opts *PkgOptions, args ...string) ([]pack.Package, error) {
	var err error
	var source string
	var sourcePackage pack.Package
	popts := []app.ProjectOption{}

	var pmanager packmanager.PackageManager
	if opts.Format != "auto" {
		pmap, err := packmanager.PackageManagers()
		if err != nil {
			return nil, errors.New("could not retrieve list of Package Managers")
		}

		pmanager = pmap[pack.PackageFormat(opts.Format)]
		if pmanager == nil {
			return nil, errors.New("invalid package format specified")
		}
	} else {
		pmanager = packmanager.G(ctx)
	}

	if len(args) == 0 {
		source, err = os.Getwd()
		if err != nil {
			return nil, err
		}
		popts = append(popts, app.WithProjectWorkdir(source))
	} else {
		source = args[0]

		// The provided argument is either a directory or a Kraftfile
		if fi, err := os.Stat(args[0]); err == nil && fi.IsDir() {
			popts = append(popts, app.WithProjectWorkdir(source))
		} else {
			if pm, compatible, err := pmanager.IsCompatible(ctx, source); err == nil && compatible {
				packages, err := pm.Catalog(ctx,
					packmanager.WithLocal(true),
					packmanager.WithName(source),
				)
				if err != nil {
					return nil, err
				}

				if len(packages) == 0 {
					return nil, fmt.Errorf("no package found for %s", source)
				} else if len(packages) > 1 {
					return nil, fmt.Errorf("multiple packages found for %s", source)
				}

				sourcePackage = packages[0]
			}
		}
	}

	var tree []*processtree.ProcessTreeItem
	var packages []pack.Package
	if sourcePackage != nil {
		cmdShellArgs, err := shellwords.Parse(strings.Join(opts.Args, " "))
		if err != nil {
			return nil, err
		}

		tree = append(tree, processtree.NewProcessTreeItem(
			sourcePackage.Name(),
			sourcePackage.Version(),
			func(ctx context.Context) error {
				var err error
				pm := packmanager.G(ctx)

				// Switch the package manager the desired format for this target
				if sourcePackage.Format().String() != "auto" {
					pm, err = pm.From(sourcePackage.Format())
					if err != nil {
						return err
					}
				}

				popts := []packmanager.PackOption{
					packmanager.PackSource(sourcePackage),
					packmanager.PackArgs(cmdShellArgs...),
					packmanager.PackInitrd(opts.Rootfs),
					packmanager.PackKConfig(!opts.NoKConfig),
					packmanager.PackName(opts.Name),
					packmanager.PackOutput(opts.Output),
				}

				if pkgs, err := pm.Pack(ctx, nil, popts...); err != nil {
					return err
				} else {
					packages = pkgs
				}

				return nil
			},
		))

		model, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(false),
				processtree.WithRenderer(
					log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
				),
			},
			tree...,
		)
		if err != nil {
			return nil, err
		}

		if err := model.Start(); err != nil {
			return nil, err
		}
	}

	fmt.Printf("%#v\n", packages[0])

	return packages, nil
}

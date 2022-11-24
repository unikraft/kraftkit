// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package packmanager

import (
	"context"
	"fmt"

	"kraftkit.sh/log"
	"kraftkit.sh/pack"
)

var packageManagers = make(map[pack.ContextKey]PackageManager)

const UmbrellaContext pack.ContextKey = "umbrella"

func PackageManagers() map[pack.ContextKey]PackageManager {
	return packageManagers
}

func RegisterPackageManager(ctxk pack.ContextKey, manager PackageManager) error {
	if _, ok := packageManagers[ctxk]; ok {
		return fmt.Errorf("package manager already registered: %s", manager.Format())
	}

	packageManagers[ctxk] = manager

	return nil
}

// UmbrellaManager is an ad-hoc package manager capable of cross managing any
// registered package manager.
type UmbrellaManager struct {
	opts *PackageManagerOptions
}

func NewUmbrellaManagerFromOptions(opts *PackageManagerOptions) (PackageManager, error) {
	umbrella := UmbrellaManager{
		opts: opts,
	}

	// Apply options on the umbrella manager to all "sub" registered package
	// managers
	for _, manager := range packageManagers {
		for _, o := range opts.opts {
			if err := o(manager.Options()); err != nil {
				return nil, err
			}
		}
	}

	return umbrella, nil
}

func (um UmbrellaManager) NewPackageFromOptions(ctx context.Context, opts *pack.PackageOptions) ([]pack.Package, error) {
	var packages []pack.Package
	for _, manager := range packageManagers {
		packed, err := manager.NewPackageFromOptions(ctx, opts)
		if err != nil {
			return packages, err
		}

		packages = append(packages, packed...)
	}

	return packages, nil
}

func (um UmbrellaManager) From(sub string) (PackageManager, error) {
	for _, manager := range packageManagers {
		if manager.Format() == sub {
			return manager, nil
		}
	}

	return nil, fmt.Errorf("unknown package manager: %s", sub)
}

func (um UmbrellaManager) ApplyOptions(pmopts ...PackageManagerOption) error {
	for _, manager := range packageManagers {
		if err := manager.ApplyOptions(pmopts...); err != nil {
			return err
		}
	}

	return nil
}

// Options allows you to view the current options.
func (um UmbrellaManager) Options() *PackageManagerOptions {
	return um.opts
}

// Update retrieves and stores locally a
func (um UmbrellaManager) Update(ctx context.Context) error {
	for _, manager := range packageManagers {
		err := manager.Update(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (um UmbrellaManager) AddSource(ctx context.Context, source string) error {
	for _, manager := range packageManagers {
		log.G(ctx).Tracef("Adding source %s via %s...", source, manager.Format())
		err := manager.AddSource(ctx, source)
		if err != nil {
			return err
		}
	}

	return nil
}

func (um UmbrellaManager) RemoveSource(ctx context.Context, source string) error {
	for _, manager := range packageManagers {
		log.G(ctx).Tracef("Removing source %s via %s...", source, manager.Format())
		err := manager.RemoveSource(ctx, source)
		if err != nil {
			return err
		}
	}

	return nil
}

// Push the resulting package to the supported registry of the implementation.
func (um UmbrellaManager) Push(ctx context.Context, path string) error {
	return fmt.Errorf("not implemented: pack.UmbrellaManager.Push")
}

// Pull a package from the support registry of the implementation.
func (um UmbrellaManager) Pull(ctx context.Context, path string, opts *pack.PullPackageOptions) ([]pack.Package, error) {
	var packages []pack.Package
	for _, manager := range packageManagers {
		log.G(ctx).Tracef("Pulling %s via %s...", path, manager.Format())
		parcel, err := manager.Pull(ctx, path, opts)
		if err != nil {
			return nil, err
		}

		packages = append(packages, parcel...)
	}

	return packages, nil
}

func (mm UmbrellaManager) Catalog(ctx context.Context, query CatalogQuery, popts ...pack.PackageOption) ([]pack.Package, error) {
	var packages []pack.Package
	for _, manager := range packageManagers {
		pack, err := manager.Catalog(ctx, query, popts...)
		if err != nil {
			return nil, err
		}

		packages = append(packages, pack...)
	}

	return packages, nil
}

// IsCompatible iterates through all package managers and returns the first
// package manager which is compatible with the provided source
func (mm UmbrellaManager) IsCompatible(ctx context.Context, source string) (PackageManager, error) {
	var err error
	var pm PackageManager
	for _, manager := range packageManagers {
		pm, err = manager.IsCompatible(ctx, source)
		if err == nil {
			return pm, nil
		}
	}

	return nil, fmt.Errorf("cannot find compatible package manager for source: %s", source)
}

func (um UmbrellaManager) Format() string {
	return string(UmbrellaContext)
}

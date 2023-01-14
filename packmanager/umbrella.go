// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package packmanager

import (
	"context"
	"fmt"

	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft/component"
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
type UmbrellaManager struct{}

func (um UmbrellaManager) From(sub string) (PackageManager, error) {
	for _, manager := range packageManagers {
		if manager.Format() == sub {
			return manager, nil
		}
	}

	return nil, fmt.Errorf("unknown package manager: %s", sub)
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

func (um UmbrellaManager) Pack(ctx context.Context, entity component.Component, opts ...PackOption) ([]pack.Package, error) {
	var ret []pack.Package

	for _, manager := range packageManagers {
		log.G(ctx).Tracef("Packing %s via %s...", entity.Name(), manager.Format())
		more, err := manager.Pack(ctx, entity, opts...)
		if err != nil {
			return nil, err
		}

		ret = append(ret, more...)
	}

	return ret, nil
}

func (um UmbrellaManager) Unpack(ctx context.Context, entity pack.Package, opts ...UnpackOption) ([]component.Component, error) {
	var ret []component.Component

	for _, manager := range packageManagers {
		log.G(ctx).Tracef("Unpacking %s via %s...", entity.Name(), manager.Format())
		more, err := manager.Unpack(ctx, entity, opts...)
		if err != nil {
			return nil, err
		}

		ret = append(ret, more...)
	}

	return ret, nil
}

func (mm UmbrellaManager) Catalog(ctx context.Context, query CatalogQuery) ([]pack.Package, error) {
	var packages []pack.Package
	for _, manager := range packageManagers {
		pack, err := manager.Catalog(ctx, query)
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

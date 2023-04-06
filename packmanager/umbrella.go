// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package packmanager

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft/component"
)

var packageManagers = make(map[pack.PackageFormat]PackageManager)

const UmbrellaFormat pack.PackageFormat = "umbrella"

func PackageManagers() map[pack.PackageFormat]PackageManager {
	return packageManagers
}

func RegisterPackageManager(ctxk pack.PackageFormat, manager PackageManager) error {
	if _, ok := packageManagers[ctxk]; ok {
		return fmt.Errorf("package manager already registered: %s", ctxk)
	}

	packageManagers[ctxk] = manager

	return nil
}

// umbrella is an ad-hoc package manager capable of cross managing any
// registered package manager.
type umbrella struct{}

// NewUmbrellaManager returns a `PackageManager` which can be used to manipulate
// multiple `PackageManager`s.  The purpose is to be able to package, unpackage,
// search and generally manipulate packages of multiple types simultaneously.
func NewUmbrellaManager() PackageManager {
	return umbrella{}
}

func (u umbrella) From(sub pack.PackageFormat) (PackageManager, error) {
	for _, manager := range packageManagers {
		if manager.Format() == sub {
			return manager, nil
		}
	}

	return nil, fmt.Errorf("unknown package manager: %s", sub)
}

func (u umbrella) Update(ctx context.Context) error {
	for _, manager := range packageManagers {
		log.G(ctx).WithFields(logrus.Fields{
			"format": manager.Format(),
		}).Tracef("updating")
		err := manager.Update(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (u umbrella) AddSource(ctx context.Context, source string) error {
	for _, manager := range packageManagers {
		log.G(ctx).WithFields(logrus.Fields{
			"format": manager.Format(),
			"source": source,
		}).Tracef("adding")
		err := manager.AddSource(ctx, source)
		if err != nil {
			return err
		}
	}

	return nil
}

func (u umbrella) RemoveSource(ctx context.Context, source string) error {
	for _, manager := range packageManagers {
		log.G(ctx).WithFields(logrus.Fields{
			"format": manager.Format(),
			"source": source,
		}).Tracef("removing")
		err := manager.RemoveSource(ctx, source)
		if err != nil {
			return err
		}
	}

	return nil
}

func (u umbrella) Pack(ctx context.Context, source component.Component, opts ...PackOption) ([]pack.Package, error) {
	var ret []pack.Package

	for _, manager := range packageManagers {
		log.G(ctx).WithFields(logrus.Fields{
			"format": manager.Format(),
			"source": source.Name(),
		}).Tracef("packing")
		more, err := manager.Pack(ctx, source, opts...)
		if err != nil {
			return nil, err
		}

		ret = append(ret, more...)
	}

	return ret, nil
}

func (u umbrella) Unpack(ctx context.Context, source pack.Package, opts ...UnpackOption) ([]component.Component, error) {
	var ret []component.Component

	for _, manager := range packageManagers {
		log.G(ctx).WithFields(logrus.Fields{
			"format": manager.Format(),
			"source": source.Name(),
		}).Tracef("unpacking")
		more, err := manager.Unpack(ctx, source, opts...)
		if err != nil {
			return nil, err
		}

		ret = append(ret, more...)
	}

	return ret, nil
}

func (u umbrella) Catalog(ctx context.Context, query CatalogQuery) ([]pack.Package, error) {
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

func (u umbrella) IsCompatible(ctx context.Context, source string) (PackageManager, bool, error) {
	if source == "" {
		return nil, false, fmt.Errorf("cannot determine compatibility of empty source")
	}

	for _, manager := range packageManagers {
		log.G(ctx).WithFields(logrus.Fields{
			"format": manager.Format(),
			"source": source,
		}).Tracef("checking compatibility")

		pm, compatible, err := manager.IsCompatible(ctx, source)
		if err == nil && compatible {
			return pm, true, nil
		} else if err != nil {
			log.G(ctx).
				WithField("format", manager.Format()).
				Tracef("package manager is not compatible because: %v", err)
		}
	}

	return nil, false, fmt.Errorf("cannot find compatible package manager for source: %s", source)
}

func (u umbrella) Format() pack.PackageFormat {
	return UmbrellaFormat
}

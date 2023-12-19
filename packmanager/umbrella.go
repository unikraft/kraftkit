// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package packmanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/log"

	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft/component"
)

// UmbrellaInstance is a singleton of our umbrella package manager.
var UmbrellaInstance *UmbrellaManager

// UmbrellaManager is an ad-hoc package manager capable of cross managing any
// registered package managers.
type UmbrellaManager struct {
	packageManagers            map[pack.PackageFormat]PackageManager
	packageManagerOpts         map[pack.PackageFormat][]any
	packageManagerConstructors map[pack.PackageFormat]NewManagerConstructor
}

// InitUmbrellaManager creates the instance of the umbrella manager singleton.
// It allows us to do dependency injection for package manager constructors.
func InitUmbrellaManager(ctx context.Context, constructors []func(*UmbrellaManager) error) error {
	if UmbrellaInstance != nil {
		return errors.New("tried to reinitialize the umbrella manager but it already exists")
	}

	umbrellaInstance, err := NewUmbrellaManager(ctx, constructors)
	if err != nil {
		return err
	}

	UmbrellaInstance = umbrellaInstance

	return nil
}

func (u UmbrellaManager) From(sub pack.PackageFormat) (PackageManager, error) {
	for _, manager := range u.packageManagers {
		if manager.Format() == sub {
			return manager, nil
		}
	}

	return nil, fmt.Errorf("unknown package manager: %s", sub)
}

func (u UmbrellaManager) Update(ctx context.Context) error {
	for _, manager := range u.packageManagers {
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

func (u UmbrellaManager) SetSources(ctx context.Context, sources ...string) error {
	for _, manager := range u.packageManagers {
		err := manager.SetSources(ctx, sources...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (u UmbrellaManager) AddSource(ctx context.Context, source string) error {
	for _, manager := range u.packageManagers {
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

func (u UmbrellaManager) Delete(ctx context.Context, qopts ...QueryOption) error {
	for _, manager := range u.packageManagers {
		log.G(ctx).
			WithField("format", manager.Format()).
			WithFields(NewQuery(qopts...).Fields()).
			Tracef("deleting")

		if err := manager.Delete(ctx, qopts...); err != nil {
			return err
		}
	}

	return nil
}

func (u UmbrellaManager) RemoveSource(ctx context.Context, source string) error {
	for _, manager := range u.packageManagers {
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

func (u UmbrellaManager) Pack(ctx context.Context, source component.Component, opts ...PackOption) ([]pack.Package, error) {
	var ret []pack.Package

	for _, manager := range u.packageManagers {
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

func (u UmbrellaManager) Unpack(ctx context.Context, source pack.Package, opts ...UnpackOption) ([]component.Component, error) {
	var ret []component.Component

	for _, manager := range u.packageManagers {
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

func (u UmbrellaManager) Catalog(ctx context.Context, qopts ...QueryOption) ([]pack.Package, error) {
	var packages []pack.Package
	for _, manager := range u.packageManagers {
		pack, err := manager.Catalog(ctx, qopts...)
		if err != nil {
			log.G(ctx).
				WithField("format", manager.Format()).
				Debugf("could not query catalog: %v", err)
			continue
		}

		packages = append(packages, pack...)
	}

	return packages, nil
}

func (u UmbrellaManager) IsCompatible(ctx context.Context, source string, qopts ...QueryOption) (PackageManager, bool, error) {
	if source == "" {
		return nil, false, fmt.Errorf("cannot determine compatibility of empty source")
	}

	for _, manager := range u.packageManagers {
		log.G(ctx).WithFields(logrus.Fields{
			"format": manager.Format(),
			"source": source,
		}).Debugf("checking compatibility")

		pm, compatible, err := manager.IsCompatible(ctx, source, qopts...)
		if err == nil && compatible {
			return pm, true, nil
		} else {
			log.G(ctx).
				WithField("format", manager.Format()).
				Debugf("package manager is not compatible because: %v", err)
		}
	}

	return nil, false, fmt.Errorf("cannot find compatible package manager for source: %s", source)
}

func (u *UmbrellaManager) PackageManagers() map[pack.PackageFormat]PackageManager {
	return u.packageManagers
}

func (u *UmbrellaManager) RegisterPackageManager(ctxk pack.PackageFormat, constructor NewManagerConstructor, opts ...any) error {
	if u.packageManagerConstructors == nil {
		u.packageManagerConstructors = make(map[pack.PackageFormat]NewManagerConstructor)
	}
	if u.packageManagerOpts == nil {
		u.packageManagerOpts = make(map[pack.PackageFormat][]any)
	}
	if u.packageManagers == nil {
		u.packageManagers = make(map[pack.PackageFormat]PackageManager)
	}

	if _, ok := u.packageManagerConstructors[ctxk]; ok {
		return fmt.Errorf("package manager already registered: %s", ctxk)
	}

	u.packageManagerConstructors[ctxk] = constructor
	u.packageManagerOpts[ctxk] = opts

	return nil
}

func (u UmbrellaManager) Format() pack.PackageFormat {
	return UmbrellaFormat
}

const UmbrellaFormat pack.PackageFormat = "umbrella"

// PackageManagers returns all package managers registered
// with the umbrella package manager.
func PackageManagers() (map[pack.PackageFormat]PackageManager, error) {
	if UmbrellaInstance == nil {
		return nil, errors.New("umbrella manager not initialized")
	}
	return UmbrellaInstance.PackageManagers(), nil
}

// WithDefaultUmbrellaManagerInContext returns a context containing the
// default umbrella package manager.
func WithDefaultUmbrellaManagerInContext(ctx context.Context) (context.Context, error) {
	if UmbrellaInstance == nil {
		return nil, errors.New("umbrella manager not initialized")
	}
	return WithManagerInContext(ctx, UmbrellaInstance), nil
}

// WithManagerInContext inserts a package manager into a context.
func WithManagerInContext(ctx context.Context, pm PackageManager) context.Context {
	return WithPackageManager(ctx, pm)
}

// NewUmbrellaManager returns a `PackageManager` which can be used to manipulate
// multiple `PackageManager`s.  The purpose is to be able to package, unpackage,
// search and generally manipulate packages of multiple types simultaneously.
// The user can pass a slice of constructors to determine which package managers
// are to be included.
func NewUmbrellaManager(ctx context.Context, constructors []func(*UmbrellaManager) error) (*UmbrellaManager, error) {
	u := &UmbrellaManager{}
	for _, reg := range constructors {
		if err := reg(u); err != nil {
			return nil, fmt.Errorf("failed registering a package manager: %w", err)
		}
	}
	for format, constructor := range u.packageManagerConstructors {
		log.G(ctx).WithField("format", format).Trace("initializing package manager")

		var opts []any

		if pmopts, ok := u.packageManagerOpts[format]; ok {
			opts = pmopts
		}

		manager, err := constructor(ctx, opts...)
		if err != nil {
			log.G(ctx).
				WithField("format", format).
				Debugf("could not initialize package manager: %v", err)
			continue
		}

		if format, ok := u.packageManagers[manager.Format()]; ok {
			return nil, fmt.Errorf("package manager already registered: %s", format)
		}

		u.packageManagers[manager.Format()] = manager
	}

	return u, nil
}

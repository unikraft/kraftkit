// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package packmanager

import (
	"context"

	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft/component"
)

// NewManagerConstructor represents the prototype that all implementing package
// managers must use during their instantiation.  This standardizes their input
// and output during "construction", particularly providing access to a
// referencable context with which they can access (within the context of
// KraftKit) the logging, IOStreams and Config subsystems.
type NewManagerConstructor func(context.Context, ...any) (PackageManager, error)

type PackageManager interface {
	// Update retrieves and stores locally a cache of the upstream registry.
	Update(context.Context) error

	// Pack turns the provided component into the distributable package.  Since
	// components can comprise of other components, it is possible to return more
	// than one package.  It is possible to disable this and "flatten" a component
	// into a single package by setting a relevant `pack.PackOption`.
	Pack(context.Context, component.Component, ...PackOption) ([]pack.Package, error)

	// Unpack turns a given package into a usable component.  Since a package can
	// compromise of a multiple components, it is possible to return multiple
	// components.
	Unpack(context.Context, pack.Package, ...UnpackOption) ([]component.Component, error)

	// Catalog returns all packages known to the manager via given query
	Catalog(context.Context, ...QueryOption) ([]pack.Package, error)

	// Set the list of sources for the package manager
	SetSources(context.Context, ...string) error

	// Add a source to the package manager
	AddSource(context.Context, string) error

	// Prune a/all packages from the host machine
	Prune(context.Context, ...QueryOption) error

	// Remove a source from the package manager
	RemoveSource(context.Context, string) error

	// IsCompatible checks whether the provided source is compatible with the
	// package manager
	IsCompatible(context.Context, string, ...QueryOption) (PackageManager, bool, error)

	// From is used to retrieve a sub-package manager.  For now, this is a small
	// hack used for the umbrella.
	From(pack.PackageFormat) (PackageManager, error)

	// Format returns the name of the implementation.
	Format() pack.PackageFormat
}

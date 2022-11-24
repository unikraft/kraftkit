// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package packmanager

import (
	"context"

	"kraftkit.sh/pack"
)

type PackageManager interface {
	// NewPackage initializes a new package
	NewPackageFromOptions(context.Context, *pack.PackageOptions) ([]pack.Package, error)

	// Options allows you to view the current options.
	Options() *PackageManagerOptions

	// ApplyOptions allows one to update the options of a package manager
	ApplyOptions(...PackageManagerOption) error

	// Update retrieves and stores locally a cache of the upstream registry.
	Update(context.Context) error

	// Push a package to the supported registry of the implementation.
	Push(context.Context, string) error

	// Pull package(s) from the supported registry of the implementation.
	Pull(context.Context, string, *pack.PullPackageOptions) ([]pack.Package, error)

	// Catalog returns all packages known to the manager via given query
	Catalog(context.Context, CatalogQuery, ...pack.PackageOption) ([]pack.Package, error)

	// Add a source to the package manager
	AddSource(context.Context, string) error

	// Remove a source from the package manager
	RemoveSource(context.Context, string) error

	// IsCompatible checks whether the provided source is compatible with the
	// package manager
	IsCompatible(context.Context, string) (PackageManager, error)

	// From is used to retrieve a sub-package manager.  For now, this is a small
	// hack used for the umbrella.
	From(string) (PackageManager, error)

	// Format returns the name of the implementation.
	Format() string
}

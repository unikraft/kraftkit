// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pack

import "context"

type Package interface {
	// Options allows you to view the current options.
	Options() *PackageOptions

	// ApplyOptions allows one to update the options of a package
	ApplyOptions(opts ...PackageOption) error

	// Determine if the provided path is a compatible media type
	Compatible(string) bool

	// Name is the simple package name
	Name() string

	// CanonicalName represents the full name which can be understood by the
	// respective package manager.
	CanonicalName() string

	// Package a package
	Pack(context.Context) error

	// Pull retreives the package artifacts given the context of the
	// PackageOptions and allows for customization of the pull via the input
	// optional PullPackageOptions
	Pull(context.Context, ...PullPackageOption) error

	// Format returns the name of the implementation.
	Format() string
}

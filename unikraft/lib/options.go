// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

type LibraryOption func(*LibraryConfig) error

// WithInternal marks the library as a part of the Unikraft core.
func WithIsInternal(internal bool) LibraryOption {
	return func(lc *LibraryConfig) error {
		lc.internal = internal
		return nil
	}
}

// WithName sets the name of this library component.
func WithName(name string) LibraryOption {
	return func(lc *LibraryConfig) error {
		lc.name = name
		return nil
	}
}

// WithSource sets the library's source which indicates where it was retrieved
// and in component context and not the origin.
func WithSource(source string) LibraryOption {
	return func(lc *LibraryConfig) error {
		lc.source = source
		return nil
	}
}

// WithVersion sets the version of this library component.
func WithVersion(version string) LibraryOption {
	return func(lc *LibraryConfig) error {
		lc.version = version
		return nil
	}
}

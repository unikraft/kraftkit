// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package pkg

import (
	"context"
	"fmt"
)

// packager is an interface for defining different mechanisms to perform a
// packaging of a unikernel.  Standardizing first the check, Packagable,
// to determine whether the provided input is capable of executing, and
// Pack, actually performing the packaging.
type packager interface {
	// String implements fmt.Stringer and returns the name of the implementing
	// builder.
	fmt.Stringer

	// Packagable determines whether the provided input is packagable by the
	// current implementation.
	Packagable(context.Context, *PkgOptions, ...string) (bool, error)

	// Pack performs the packaging based on the determined implementation.
	Pack(context.Context, *PkgOptions, ...string) error
}

// packagers is the list of built-in packagers which are checked
// sequentially for capability.  The first to test positive via Packagable
// is used with the controller.
func packagers() []packager {
	return []packager{
		&packagerKraftfileUnikraft{},
		&packagerKraftfileRuntime{},
	}
}

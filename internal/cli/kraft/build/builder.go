// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"fmt"
)

// builder is an interface for defining different mechanisms to perform a build
// of a unikernel.  Standardizing first the check, Runnable, to determine
// whether the provided input is capable of executing, and Prepare, actually
// performing the preparation of the Machine specification for the controller.
type builder interface {
	// String implements fmt.Stringer and returns the name of the implementing
	// builder.
	fmt.Stringer

	// Buildable determines whether the provided input can be constructed for the
	// given implementation.
	Buildable(context.Context, *BuildOptions, ...string) (bool, error)

	// Prepare performs any pre-emptive operations that are necessary before
	// performing the build.
	Prepare(context.Context, *BuildOptions, ...string) error

	// Build performs the actual construction of the unikernel given the provided
	// inputs for the given implementation.
	Build(context.Context, *BuildOptions, ...string) error

	// Statistics returns the statistics for the build.
	Statistics(context.Context, *BuildOptions, ...string) error
}

// builders is the list of built-in builders which are checked sequentially for
// capability.  The first to test positive via Runnable is used with the
// controller.
func builders() []builder {
	return []builder{
		&builderKraftfileUnikraft{},
		&builderKraftfileRuntime{},
	}
}

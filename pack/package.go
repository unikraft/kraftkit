// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pack

import (
	"context"

	"kraftkit.sh/unikraft"
)

type Package interface {
	unikraft.Nameable

	// Metadata returns any additional metadata associated with this package.
	Metadata() any

	// Push the package to a remotely retrievable destination.
	Push(context.Context, ...PushOption) error

	// Pull retreives the package from a remotely retrievable location.
	Pull(context.Context, ...PullOption) error

	// Format returns the name of the implementation.
	Format() string
}

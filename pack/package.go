// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pack

import (
	"context"
	"fmt"

	"kraftkit.sh/unikraft"
)

type PackageFormat string

var _ fmt.Stringer = (*PackageFormat)(nil)

func (pf PackageFormat) String() string {
	return string(pf)
}

type Package interface {
	unikraft.Nameable

	// Metadata returns any additional metadata associated with this package.
	Metadata() any

	// Push the package to a remotely retrievable destination.
	Push(context.Context, ...PushOption) error

	// Pull retreives the package from a remotely retrievable location.
	Pull(context.Context, ...PullOption) error

	// Deletes package available locally.
	Delete(context.Context, string) error

	// Format returns the name of the implementation.
	Format() PackageFormat
}

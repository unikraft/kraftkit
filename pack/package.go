// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pack

import (
	"context"
	"fmt"
	"time"

	"kraftkit.sh/internal/tableprinter"
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
	Metadata() interface{}

	// Columns is a subset of Metadata that is displayed to the user and can be
	// also collated or parsed to made easier-to-read.
	Columns() []tableprinter.Column

	// Push the package to a remotely retrievable destination.
	Push(context.Context, ...PushOption) error

	// Pull retreives the package from a remotely retrievable location.
	Pull(context.Context, ...PullOption) error

	// Unpack the package into the specified directory.
	Unpack(context.Context, string) error

	// Save the state of package.
	Save(context.Context) error

	// Deletes package available locally.
	Delete(context.Context) error

	// PulledAt is an attribute of a package to indicate when (and if) it was
	// last retrieved.
	PulledAt(context.Context) (bool, time.Time, error)

	// Format returns the name of the implementation.
	Format() PackageFormat
}

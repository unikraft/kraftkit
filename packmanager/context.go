// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package packmanager

import (
	"context"
)

var (
	// G is an alias for FromContext.
	//
	// We may want to define this locally to a package to get package tagged
	// package manager.
	G = FromContext

	// PM is the system-access umbrella package manager.
	PM = UmbrellaManager{}
)

// contextKey is used to retrieve the package manager from the context.
type contextKey struct{}

// WithPackageManager returns a new context with the provided package manager.
func WithPackageManager(ctx context.Context, pm PackageManager) context.Context {
	return context.WithValue(ctx, contextKey{}, pm)
}

// FromContext returns the package manager in the context, or access to the
// umbrella package manager which iterates over all registered package managers.
func FromContext(ctx context.Context) PackageManager {
	l := ctx.Value(contextKey{})

	if l == nil {
		return PM
	}

	return l.(PackageManager)
}

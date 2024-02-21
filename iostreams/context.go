// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package iostreams

import (
	"context"
)

var (
	// G is an alias for FromContext.
	//
	// We may want to define this locally to a package to get package tagged
	// iostreams.
	G = FromContext

	// L is the system IO stream.
	IO = System()
)

// contextKey is used to retrieve the logger from the context.
type contextKey struct{}

// WithIOStreams returns a new context with the provided logger. Use in
// combination with logger.WithField(s) for great effect.
func WithIOStreams(ctx context.Context, iostreams *IOStreams) context.Context {
	return context.WithValue(ctx, contextKey{}, iostreams)
}

// FromContext returns the logger kraftkit in the context, or an inert logger
// that will not log anything.
func FromContext(ctx context.Context) *IOStreams {
	if ctx == nil {
		return IO
	}

	l := ctx.Value(contextKey{})

	if l == nil {
		return IO
	}

	return l.(*IOStreams)
}

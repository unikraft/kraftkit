// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package unikraft

import "context"

// contextKey is used to retrieve the logger from the context.
type contextKey struct{}

// Context is a general-purpose context container for use within the Unikraft
// package.  It mimics the environmental variables within Unikraft's main build
// system.
type Context struct {
	UK_NAME   string
	UK_BASE   string
	BUILD_DIR string
}

// WithContext
func WithContext(ctx context.Context, val *Context) context.Context {
	return context.WithValue(ctx, contextKey{}, val)
}

func FromContext(ctx context.Context) *Context {
	uk, ok := ctx.Value(contextKey{}).(*Context)
	if !ok {
		return nil
	}

	return uk
}

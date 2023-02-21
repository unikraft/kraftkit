// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package config

import (
	"context"
)

// contextKey is used to retrieve the logger from the context.
type contextKey struct{}

// WithConfigManager returns a new context with the provided logger. Use in
// combination with logger.WithField(s) for great effect.
func WithConfigManager[C any](ctx context.Context, cfgm *ConfigManager[C]) context.Context {
	return context.WithValue(ctx, contextKey{}, cfgm)
}

// FromContext returns the Config Manager for kraftkit in the context, or an
// inert configuration that results in default values.
func M[T any](ctx context.Context) *ConfigManager[T] {
	l := ctx.Value(contextKey{})

	if l == nil {
		l, _ := NewConfigManager(new(T))
		return l
	}

	return l.(*ConfigManager[T])
}

// ConfigFromContext returns the config for kraftkit in the context, or an inert
// configuration that results in default values.
func G[T any](ctx context.Context) *T {
	return M[T](ctx).Config
}

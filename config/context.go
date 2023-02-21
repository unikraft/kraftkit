// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package config

import (
	"context"
)

var (
	// G is an alias for FromContext.
	//
	// We may want to define this locally to a package to get package tagged
	// config.
	G = ConfigFromContext

	// M is an alias for FromContext
	M = FromContext

	// C is the default configuration manager
	C, _ = NewConfigManager()
)

// contextKey is used to retrieve the logger from the context.
type contextKey struct{}

// WithConfigManager returns a new context with the provided logger. Use in
// combination with logger.WithField(s) for great effect.
func WithConfigManager(ctx context.Context, cfgm *ConfigManager) context.Context {
	return context.WithValue(ctx, contextKey{}, cfgm)
}

// FromContext returns the Config Manager for kraftkit in the context, or an
// inert configuration that results in default values.
func FromContext(ctx context.Context) *ConfigManager {
	l := ctx.Value(contextKey{})

	if l == nil {
		return C
	}

	return l.(*ConfigManager)
}

// ConfigFromContext returns the config for kraftkit in the context, or an inert
// configuration that results in default values.
func ConfigFromContext(ctx context.Context) *Config {
	return FromContext(ctx).Config
}

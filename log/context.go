// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package log

import (
	"context"

	"github.com/sirupsen/logrus"
)

var (
	// G is an alias for FromContext.
	//
	// We may want to define this locally to a package to get package tagged log
	// messages.
	G = FromContext

	// L is the global logger.
	L = logrus.StandardLogger()
)

// contextKey is used to retrieve the logger from the context.
type contextKey struct{}

// WithLogger returns a new context with the provided logger. Use in
// combination with logger.WithField(s) for great effect.
func WithLogger(ctx context.Context, logger *logrus.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext returns the logger kraftkit in the context, or an inert logger
// that will not log anything.
func FromContext(ctx context.Context) *logrus.Logger {
	l, ok := ctx.Value(contextKey{}).(*logrus.Logger)
	if !ok || l == nil {
		return L
	}

	return l
}

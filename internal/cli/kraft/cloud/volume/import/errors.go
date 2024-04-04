// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package vimport

import "errors"

// Loggable is a type of error that can be asserted for behaviour, and signals
// whether an error should be logged when handled.
// It is particularly useful for communicating to a caller that the error was
// already reported (e.g. logged) in a callee.
type Loggable interface {
	error
	Loggable() bool
}

// NotLoggable marks an error as not loggable.
func NotLoggable(err error) error {
	return noLogError{e: err}
}

// IsLoggable returns whether err should be logged when handled.
// All errors are considered loggable unless explicitly marked otherwise.
func IsLoggable(err error) bool {
	lerr := (Loggable)(nil)
	if impl := errors.As(err, &lerr); !impl {
		return true
	}
	return lerr.Loggable()
}

// noLogError is an error type that should not be logged when handled.
type noLogError struct {
	e error
}

var _ Loggable = (*noLogError)(nil)

// Loggable implements Loggable.
func (noLogError) Loggable() bool {
	return false
}

// Error implements the error interface.
func (e noLogError) Error() string {
	return e.e.Error()
}

// Unwrap implements the Unwrap part of the error interface.
func (e noLogError) Unwrap() error {
	return e.e
}

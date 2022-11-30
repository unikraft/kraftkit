// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package errs

import "errors"

var (
	// ErrNotFound is returned when an object is not found
	ErrNotFound = errors.New("not found")

	// ErrInvalid is returned when a compose project is invalid
	ErrInvalid = errors.New("invalid")

	// ErrUnsupported is returned when a compose project uses an unsupported attribute
	ErrUnsupported = errors.New("unsupported")

	// ErrIncompatible is returned when a compose project uses an incompatible attribute
	ErrIncompatible = errors.New("incompatible")
)

// IsNotFoundError returns true if the unwrapped error is ErrNotFound
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsInvalidError returns true if the unwrapped error is ErrInvalid
func IsInvalidError(err error) bool {
	return errors.Is(err, ErrInvalid)
}

// IsUnsupportedError returns true if the unwrapped error is ErrUnsupported
func IsUnsupportedError(err error) bool {
	return errors.Is(err, ErrUnsupported)
}

// IsUnsupportedError returns true if the unwrapped error is ErrIncompatible
func IsIncompatibleError(err error) bool {
	return errors.Is(err, ErrIncompatible)
}

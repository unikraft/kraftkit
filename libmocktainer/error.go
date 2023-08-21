// SPDX-License-Identifier: Apache-2.0
// Copyright 2014 Docker, Inc.
// Copyright 2023 Unikraft GmbH and The KraftKit Authors

package libmocktainer

import "errors"

var (
	ErrExist      = errors.New("container with given ID already exists")
	ErrInvalidID  = errors.New("invalid container ID format")
	ErrNotExist   = errors.New("container does not exist")
	ErrRunning    = errors.New("container still running")
	ErrNotRunning = errors.New("container not running")
)

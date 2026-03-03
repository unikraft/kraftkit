// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package cpio

type CpioCreateOptions struct {
	allRoot bool
}

type CpioCreateOption func(*CpioCreateOptions) error

// WithAllRoot toggles whether all files permissions should be set to root:root
// instead of the original file permissions.
func WithAllRoot(allRoot bool) CpioCreateOption {
	return func(co *CpioCreateOptions) error {
		co.allRoot = allRoot
		return nil
	}
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package packmanager

// UnpackOptions contains the list of options which can be used to unpackage an
// a component.
type UnpackOptions struct {
	workdir string
}

// UnpackOption is an option function which is used to modify UnpackOptions.
type UnpackOption func(*UnpackOptions) error

// WithUnpackWorkdir sets the directory to unpack the package to
func WithUnpackWorkdir(workdir string) UnpackOption {
	return func(uopts *UnpackOptions) error {
		uopts.workdir = workdir
		return nil
	}
}

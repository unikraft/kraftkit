// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package oci

type ImageOption func(*Image) error

// WithWorkdir specifies the working directory of the image.
func WithWorkdir(workdir string) ImageOption {
	return func(image *Image) error {
		image.workdir = workdir
		return nil
	}
}

// WithAutoSave atomicizes the image as operations occur on its body.
func WithAutoSave(autoSave bool) ImageOption {
	return func(image *Image) error {
		image.autoSave = autoSave
		return nil
	}
}

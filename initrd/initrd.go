// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import "context"

const (
	// DefaultInitramfsFileName is the default filename used when creating or
	// serializing a CPIO archive.
	DefaultInitramfsFileName = "initramfs.%s"

	// DefaultInitramfsArchFileName is the default filename used when creating
	// or serializing a rootfs archive based on a specific architecture
	DefaultInitramfsArchFileName = "initramfs-%s.%s"
)

// Initrd is an interface that is used to allow for different underlying
// implementations to construct a CPIO archive.
type Initrd interface {
	// Name of the initramfs builder.
	Name() string

	// Build the rootfs and return the location of the result or error.
	Build(context.Context) (string, error)

	// Options returns the options used to construct this Initrd.
	Options() InitrdOptions

	// All environment variables that are set within.
	Env() []string

	// All arguments that are passed to the initramfs.
	Args() []string
}

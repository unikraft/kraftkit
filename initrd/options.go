// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

type InitrdOptions struct {
	output   string
	cacheDir string
	arch     string
}

type InitrdOption func(*InitrdOptions) error

// WithOutput sets the location of the output location of the resulting CPIO
// archive file.
func WithOutput(output string) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.output = output
		return nil
	}
}

// WithCacheDir sets the path of an internal location that's used during the
// serialization of the initramfs as a mechanism for storing temporary files
// used as cache.
func WithCacheDir(dir string) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.cacheDir = dir
		return nil
	}
}

// WithArchitecture sets the architecture of the file contents of binaries in
// the initramfs.  Files may not always be architecture specific, this option
// simply indicates the target architecture if any binaries are compiled by the
// implementing initrd builder.
func WithArchitecture(arch string) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.arch = arch
		return nil
	}
}

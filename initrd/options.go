// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import "fmt"

type InitrdOptions struct {
	arch       string
	cacheDir   string
	compress   bool
	keepOwners bool
	output     string
	fsType     FsType
	workdir    string
}

// Whether the resulting archive file should be compressed. (CPIO only)
func (opts InitrdOptions) Compress() bool {
	return opts.compress
}

// The output location of the resulting archive file.
func (opts InitrdOptions) Output() string {
	return opts.output
}

// The cache directory used during the serialization of the initramfs.
func (opts InitrdOptions) CacheDir() string {
	return opts.cacheDir
}

// The architecture of the file contents of binaries in the initramfs.
func (opts InitrdOptions) Architecture() string {
	return opts.arch
}

// The working directory of the initramfs builder.
func (opts InitrdOptions) Workdir() string {
	return opts.workdir
}

type InitrdOption func(*InitrdOptions) error

// WithCompression sets the compression of the resulting archive file.
// (CPIO only)
func WithCompression(compress bool) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.compress = compress
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

// WithCacheDir sets the path of an internal location that's used during the
// serialization of the initramfs as a mechanism for storing temporary files
// used as cache.
func WithCacheDir(dir string) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.cacheDir = dir
		return nil
	}
}

// WithKeepOwners sets whether the resulting archive file should keep the
// owners of the files in the initramfs.
func WithKeepOwners(keep bool) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.keepOwners = keep
		return nil
	}
}

// WithOutput sets the location of the resulting archive file.
func WithOutput(output string) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.output = output
		return nil
	}
}

// WithOutputType sets the output type of the resulting root filesystem.
func WithOutputType(fsType FsType) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.fsType = fsType
		return nil
	}
}

// WithWorkdir sets the working directory of the initramfs builder.  This is
// used as a mechanism for storing temporary files and directories during the
// serialization of the initramfs.
func WithWorkdir(dir string) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.workdir = dir
		return nil
	}
}

type FsType string

const (
	FsTypeCpio    = FsType("cpio")
	FsTypeErofs   = FsType("erofs")
	FsTypeFile    = FsType("file")
	FsTypeUnknown = FsType("unknown")
)

var _ fmt.Stringer = (*FsType)(nil)

// String implements fmt.Stringer
func (fsType FsType) String() string {
	return string(fsType)
}

// FsTypes returns the list of possible fsTypes.
func FsTypes() []FsType {
	return []FsType{
		FsTypeCpio,
		FsTypeErofs,
	}
}

// FsTypeNames returns the string representation of all possible
// fsType implementations.
func FsTypeNames() []string {
	types := []string{}
	for _, name := range FsTypes() {
		types = append(types, name.String())
	}

	return types
}

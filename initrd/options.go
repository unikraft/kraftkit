// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"fmt"

	"kraftkit.sh/config"
)

type InitrdBuildSecret struct {
	Name string
	File string
	Env  string
}

type InitrdOptions struct {
	arch         string
	auths        map[string]config.AuthConfig
	buildArgs    map[string]*string
	buildTarget  string
	buildSecrets map[string]InitrdBuildSecret
	cacheDir     string
	compress     bool
	fsType       FsType
	keepOwners   bool
	output       string
	rootfsPath   string
	workdir      string
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

// The rootfs path for the initramfs.
func (opts InitrdOptions) RootfsPath() string {
	return opts.rootfsPath
}

// The build arguments that may be used by certain initrd builders.
func (opts InitrdOptions) BuildArgs() map[string]*string {
	return opts.buildArgs
}

// The build target that may be used by certain initrd builders.
func (opts InitrdOptions) BuildTarget() string {
	return opts.buildTarget
}

// The build secrets that may be used by certain initrd builders.
func (opts InitrdOptions) BuildSecrets() map[string]InitrdBuildSecret {
	return opts.buildSecrets
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
		if fsType == "" {
			return nil
		}
		for _, validType := range FsTypes() {
			if fsType == validType {
				opts.fsType = fsType
				return nil
			}
		}
		return fmt.Errorf("invalid output type '%s', valid types are: %v", fsType, FsTypeNames())
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

// WithRootfsPath sets the rootfs path for the initramfs.
func WithRootfsPath(path string) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.rootfsPath = path
		return nil
	}
}

// WithBuildArgs sets build arguments that may be used by certain initrd
// builders, such as Dockerfile-based ones.
func WithBuildArgs(args map[string]*string) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.buildArgs = args
		return nil
	}
}

// WithBuildTarget sets the build target that may be used by certain initrd
// builders, such as Dockerfile-based ones.
func WithBuildTarget(target string) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.buildTarget = target
		return nil
	}
}

// WithBuildSecrets sets build secrets that may be used by certain initrd
// builders, such as Dockerfile-based ones.
func WithBuildSecrets(secrets map[string]InitrdBuildSecret) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.buildSecrets = secrets
		return nil
	}
}

// WithAuths sets the authentication configurations for OCI registries.
func WithAuths(auths map[string]config.AuthConfig) InitrdOption {
	return func(opts *InitrdOptions) error {
		opts.auths = auths
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
		FsTypeFile,
		FsTypeUnknown,
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

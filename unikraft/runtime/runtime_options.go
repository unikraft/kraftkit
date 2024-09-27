// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package runtime

// RuntimeOption are method-driven options that are used to configure
// the instantiation of a pre-built unikernel application "runtime".
type RuntimeOption func(*Runtime) error

// WithName sets the name of the runtime.
func WithName(name string) RuntimeOption {
	return func(runtime *Runtime) error {
		runtime.name = name
		return nil
	}
}

// WithSource sets the source location of the runtime
func WithSource(source string) RuntimeOption {
	return func(runtime *Runtime) error {
		runtime.source = source
		return nil
	}
}

// WithRootfs sets the rootfs to be mounted to the runtime
func WithRootfs(rootfs string) RuntimeOption {
	return func(runtime *Runtime) error {
		runtime.rootfs = rootfs
		return nil
	}
}

// WithKernel sets the specific path to the runtime.
func WithKernel(kernel string) RuntimeOption {
	return func(runtime *Runtime) error {
		runtime.kernel = kernel
		return nil
	}
}

// WithPlatform sets the platform of the runtime.
func WithPlatform(platform string) RuntimeOption {
	return func(runtime *Runtime) error {
		runtime.platform = platform
		return nil
	}
}

// WithArchitecture sets the architecture of the runtime.
func WithArchitecture(architecture string) RuntimeOption {
	return func(runtime *Runtime) error {
		runtime.architecture = architecture
		return nil
	}
}

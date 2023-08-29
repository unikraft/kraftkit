// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package elfloader

// ELFLoaderPrebuiltOption is method-driven options that are used to configure
// the instantiation of ELFLoader application.
type ELFLoaderPrebuiltOption func(*ELFLoader) error

// WithName sets the name of the ELFLoader application.
func WithName(name string) ELFLoaderPrebuiltOption {
	return func(elfloader *ELFLoader) error {
		elfloader.name = name
		return nil
	}
}

// WithSource sets the source location of the ELFLoader application.
func WithSource(source string) ELFLoaderPrebuiltOption {
	return func(elfloader *ELFLoader) error {
		elfloader.source = source
		return nil
	}
}

// WithRootfs sets the rootfs to be mounted to the ELFLoader application.
func WithRootfs(rootfs string) ELFLoaderPrebuiltOption {
	return func(elfloader *ELFLoader) error {
		elfloader.rootfs = rootfs
		return nil
	}
}

// WithKernel sets the specific path to the ELFLoader unikernel.
func WithKernel(kernel string) ELFLoaderPrebuiltOption {
	return func(elfloader *ELFLoader) error {
		elfloader.kernel = kernel
		return nil
	}
}

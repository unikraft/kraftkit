// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package target

import (
	"kraftkit.sh/initrd"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
)

// TargetOption is a function that modifies a TargetConfig.
type TargetOption func(*TargetConfig)

// WithName sets the name of the target.
func WithName(name string) TargetOption {
	return func(tc *TargetConfig) {
		tc.name = name
	}
}

// WithVersion sets the version of the target.
func WithArchitecture(arch arch.Architecture) TargetOption {
	return func(tc *TargetConfig) {
		tc.architecture = arch
	}
}

// WithPlatform sets the platform of the target.
func WithPlatform(platform plat.Platform) TargetOption {
	return func(tc *TargetConfig) {
		tc.platform = platform
	}
}

// WithKConfig sets the kconfig of the target.
func WithKConfig(kconfig kconfig.KeyValueMap) TargetOption {
	return func(tc *TargetConfig) {
		tc.kconfig = kconfig
	}
}

// WithKernel sets the kernel of the target.
func WithKernel(kernel string) TargetOption {
	return func(tc *TargetConfig) {
		tc.kernel = kernel
	}
}

// WithKernelDbg sets the kernel debug of the target.
func WithKernelDbg(kernelDbg string) TargetOption {
	return func(tc *TargetConfig) {
		tc.kernelDbg = kernelDbg
	}
}

// WithInitrd sets the initrd of the target.
func WithInitrd(initrd initrd.Initrd) TargetOption {
	return func(tc *TargetConfig) {
		tc.initrd = initrd
	}
}

// WithCommand sets the command of the target.
func WithCommand(command []string) TargetOption {
	return func(tc *TargetConfig) {
		tc.command = command
	}
}

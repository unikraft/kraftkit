// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package plat

import "kraftkit.sh/kconfig"

// PlatformOption is a function that modifies a PlatformConfig.
type PlatformOption func(*PlatformConfig)

// WithName sets the name of the platform.
func WithName(name string) PlatformOption {
	return func(pc *PlatformConfig) {
		pc.name = name
	}
}

// WithVersion sets the version of the platform.
func WithVersion(version string) PlatformOption {
	return func(pc *PlatformConfig) {
		pc.version = version
	}
}

// WithSource sets the source of the platform.
func WithSource(source string) PlatformOption {
	return func(pc *PlatformConfig) {
		pc.source = source
	}
}

// WithPath sets the path of the platform.
func WithPath(path string) PlatformOption {
	return func(pc *PlatformConfig) {
		pc.path = path
	}
}

// WithInternal sets the internal flag of the platform.
func WithInternal(internal bool) PlatformOption {
	return func(pc *PlatformConfig) {
		pc.internal = internal
	}
}

// WithKConfig sets the kconfig of the platform.
func WithKConfig(kconfig kconfig.KeyValueMap) PlatformOption {
	return func(pc *PlatformConfig) {
		pc.kconfig = kconfig
	}
}

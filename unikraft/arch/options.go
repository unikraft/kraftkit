// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package arch

import (
	"kraftkit.sh/kconfig"
)

// ArchitectureOption is a function that modifies a ArchitectureConfig.
type ArchitectureOption func(*ArchitectureConfig)

// WithName sets the name of the architecture.
func WithName(name string) ArchitectureOption {
	return func(ac *ArchitectureConfig) {
		ac.name = name
	}
}

// WithKConfig sets the kconfig of the architecture.
func WithKConfig(kconfig kconfig.KeyValueMap) ArchitectureOption {
	return func(ac *ArchitectureConfig) {
		ac.kconfig = kconfig
	}
}

// WithVersion sets the version of the architecture.
func WithVersion(version string) ArchitectureOption {
	return func(ac *ArchitectureConfig) {
		ac.version = version
	}
}

// WithSource sets the source of the architecture.
func WithSource(source string) ArchitectureOption {
	return func(ac *ArchitectureConfig) {
		ac.source = source
	}
}

// WithPath sets the path of the architecture.
func WithPath(path string) ArchitectureOption {
	return func(ac *ArchitectureConfig) {
		ac.path = path
	}
}

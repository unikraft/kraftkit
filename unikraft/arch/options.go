// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package arch

import (
	"kraftkit.sh/kconfig"
)

// ArchitectureOption is a function that modifies a ArchitectureConfig.
type ArchitectureOption func(*ArchitectureConfig) error

// WithName sets the name of the architecture.
func WithName(name string) ArchitectureOption {
	return func(ac *ArchitectureConfig) error {
		ac.name = name
		return nil
	}
}

// WithKConfig sets the kconfig of the architecture.
func WithKConfig(kconfig kconfig.KeyValueMap) ArchitectureOption {
	return func(ac *ArchitectureConfig) error {
		ac.kconfig = kconfig
		return nil
	}
}

// WithVersion sets the version of the architecture.
func WithVersion(version string) ArchitectureOption {
	return func(ac *ArchitectureConfig) error {
		ac.version = version
		return nil
	}
}

// WithSource sets the source of the architecture.
func WithSource(source string) ArchitectureOption {
	return func(ac *ArchitectureConfig) error {
		ac.source = source
		return nil
	}
}

// WithPath sets the path of the architecture.
func WithPath(path string) ArchitectureOption {
	return func(ac *ArchitectureConfig) error {
		ac.path = path
		return nil
	}
}

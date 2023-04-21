// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package core

import "kraftkit.sh/kconfig"

// UnikraftOption is a function that modifies a UnikraftConfig.
type UnikraftOption func(*UnikraftConfig) error

// WithName sets the name of the unikraft.
func WithVersion(version string) UnikraftOption {
	return func(uc *UnikraftConfig) error {
		uc.version = version
		return nil
	}
}

// WithSource sets the source of the unikraft.
func WithSource(source string) UnikraftOption {
	return func(uc *UnikraftConfig) error {
		uc.source = source
		return nil
	}
}

// WithPath sets the path of the unikraft.
func WithPath(path string) UnikraftOption {
	return func(uc *UnikraftConfig) error {
		uc.path = path
		return nil
	}
}

// WithKConfig sets the kconfig of the unikraft.
func WithKConfig(kconfig kconfig.KeyValueMap) UnikraftOption {
	return func(uc *UnikraftConfig) error {
		uc.kconfig = kconfig
		return nil
	}
}

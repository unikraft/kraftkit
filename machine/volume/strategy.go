// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package volume

import (
	"context"

	volumev1alpha1 "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/kconfig"
)

// NewStrategyConstructor is a prototype for the instantiation function of a
// platform driver implementation.
type NewStrategyConstructor[T any] func(context.Context, ...any) (T, error)

// strategies contains the map of registered strategies, whether provided as
// part from builtins and are host specific, or those that have been registered
// dynamically at runtime.
var strategies = make(map[string]*Strategy)

// DefaultStrategyName return the name of the default strategy of the platform.
func DefaultStrategyName() string {
	return defaultStrategyName
}

// Strategy represents canonical reference of a machine driver and their
// platform.
type Strategy struct {
	IsCompatible      func(string, kconfig.KeyValueMap) (bool, error)
	NewVolumeV1alpha1 NewStrategyConstructor[volumev1alpha1.VolumeService]
}

// Strategies returns the list of registered platform implementations.
func Strategies() map[string]*Strategy {
	base := hostSupportedStrategies()
	for name, driverInfo := range strategies {
		base[name] = driverInfo
	}

	return base
}

// DriverNames returns the list of registered platform driver implementation
// names.
func DriverNames() []string {
	ret := []string{}
	for plat := range Strategies() {
		ret = append(ret, plat)
	}

	return ret
}

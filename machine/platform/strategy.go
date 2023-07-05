// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package platform

import (
	"context"

	_ "kraftkit.sh/api"
	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
)

// strategies contains the map of registered strategies, whether provided as
// part from builtins and are host specific, or those that have been registered
// dynamically at runtime.
var strategies = make(map[Platform]*Strategy)

// NewStrategyConstructor is a prototype for the instantiation function of a
// platform driver implementation.
type NewStrategyConstructor[T any] func(context.Context, ...any) (T, error)

// Strategy represents canonical reference of a machine driver and their
// platform.
type Strategy struct {
	Name               string
	Platform           Platform
	NewMachineV1alpha1 NewStrategyConstructor[machinev1alpha1.MachineService]
}

// Strategies returns the list of registered platform implementations.
func Strategies() map[Platform]*Strategy {
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

	for alias, plat := range PlatformsByName() {
		if _, ok := Strategies()[plat]; ok {
			ret = append(ret, alias)
		}
	}

	return ret
}

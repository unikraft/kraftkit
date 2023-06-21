// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"context"
	"fmt"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/packmanager"
)

// runner is an interface for defining different mechanisms to execute the
// provided unikernel.  Standardizing first the check, Runnable, to determine
// whether the provided input is capable of executing, and Prepare, actually
// performing the preparation of the Machine specification for the controller.
type runner interface {
	// String implements fmt.Stringer and returns the name of the implementing
	// runner.
	fmt.Stringer

	// Runnable checks whether the provided configuration is runnable.
	Runnable(context.Context, *Run, ...string) (bool, error)

	// Prepare the provided configuration into a machine specification ready for
	// execution by the controller.
	Prepare(context.Context, *Run, *machineapi.Machine, ...string) error
}

// runners is the list of built-in runners which are checked sequentially for
// capability.  The first to test positive via Runnable is used with the
// controller.
func runners() map[string]runner {
	r := map[string]runner{
		// "api":     &runnerApi{},
		"linuxu":  &runnerLinuxu{},
		"kernel":  &runnerKernel{},
		"project": &runnerProject{},
	}

	for k, pm := range packmanager.PackageManagers() {
		r[string(k)] = &runnerPackage{
			pm: pm,
		}
	}

	return r
}

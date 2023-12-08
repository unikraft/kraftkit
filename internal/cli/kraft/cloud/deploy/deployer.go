// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"fmt"

	kraftcloudinstances "sdk.kraft.cloud/instances"
)

// deployer is an interface for defining different mechanisms to perform a the
// deployment of a context.  Standardizing first the check, Deployable, to
// determine whether the provided input is capable of deploying, and deploy,
// actually performing the deployment.
type deployer interface {
	// String implements fmt.Stringer and returns the name of the implementing
	// builder.
	fmt.Stringer

	// Deployable determines whether the provided input is deployable by the
	// current implementation.
	Deployable(context.Context, *DeployOptions, ...string) (bool, error)

	// Deploy performs the deployment based on the determined implementation.
	Deploy(context.Context, *DeployOptions, ...string) ([]kraftcloudinstances.Instance, error)
}

// deployers is the list of built-in deployers which are checked
// sequentially for capability.  The first to test positive via Packagable
// is used with the controller.
func deployers() []deployer {
	return []deployer{
		&deployerKraftfileRuntime{},
		&deployerKraftfileUnikraft{},
	}
}

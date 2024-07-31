// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

//go:build !xen
// +build !xen

package xen

import (
	"context"
	"fmt"

	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
)

func NewMachineV1alpha1Service(ctx context.Context) (machinev1alpha1.MachineService, error) {
	return nil, fmt.Errorf("xen is not supported")
}

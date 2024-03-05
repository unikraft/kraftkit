// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"context"
	"fmt"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
)

// CheckPorts is a utility method used to throw an error if the supplied machine
// has assigned ports which are already used by an existing, running machine,
// which is accessible through the supplied machine controller.
func CheckPorts(ctx context.Context, controller machineapi.MachineService, machine *machineapi.Machine) error {
	existingMachines, err := controller.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return fmt.Errorf("getting list of existing machines: %w", err)
	}

	for _, existingMachine := range existingMachines.Items {
		for _, existingPort := range existingMachine.Spec.Ports {
			for _, newPort := range machine.Spec.Ports {
				if existingPort.HostIP == newPort.HostIP && existingPort.HostPort == newPort.HostPort && existingMachine.Status.State == machineapi.MachineStateRunning {
					return fmt.Errorf("port %s:%d is already in use by %s", existingPort.HostIP, existingPort.HostPort, existingMachine.Name)
				}
			}
		}
	}

	return nil
}

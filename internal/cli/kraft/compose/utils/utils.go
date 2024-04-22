// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"context"

	"kraftkit.sh/compose"
	"kraftkit.sh/internal/cli/kraft/remove"
	"kraftkit.sh/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	composeapi "kraftkit.sh/api/compose/v1"
	machineapi "kraftkit.sh/api/machine/v1alpha1"
	mplatform "kraftkit.sh/machine/platform"
)

func RemoveOrphans(ctx context.Context, project *compose.Project) error {
	composeController, err := compose.NewComposeProjectV1(ctx)
	if err != nil {
		return err
	}

	embeddedProject, err := composeController.Get(ctx, &composeapi.Compose{
		ObjectMeta: metav1.ObjectMeta{
			Name: project.Name,
		},
	})
	if err != nil {
		return err
	}

	machineController, err := mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	if err != nil {
		return err
	}

	machines, err := machineController.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return err
	}

	orphanMachines := []string{}
	for _, machine := range embeddedProject.Status.Machines {
		isService := false
		for _, service := range project.Services {
			if service.Name == machine.Name {
				isService = true
				break
			}
		}

		for _, m := range machines.Items {
			if m.Name == machine.Name {
				if !isService && m.Status.State == machineapi.MachineStateRunning {
					orphanMachines = append(orphanMachines, machine.Name)
				}
			}
		}
	}

	if len(orphanMachines) == 0 {
		return nil
	}

	log.G(ctx).Info("removing orphan machines...")
	removeOptions := remove.RemoveOptions{
		Platform: "auto",
	}

	return removeOptions.Run(ctx, orphanMachines)
}

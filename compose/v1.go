// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package compose

import (
	"context"
	"path/filepath"

	zip "api.zip"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	composev1 "kraftkit.sh/api/compose/v1"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/store"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	mplatform "kraftkit.sh/machine/platform"
)

type v1ComposeProject struct{}

func NewComposeProjectV1(ctx context.Context, opts ...any) (composev1.ComposeService, error) {
	embeddedStore, err := store.NewEmbeddedStore[composev1.ComposeSpec, composev1.ComposeStatus](
		filepath.Join(
			config.G[config.KraftKit](ctx).RuntimeDir,
			"composev1",
		),
	)
	if err != nil {
		return nil, err
	}

	service := &v1ComposeProject{}

	return composev1.NewComposeServiceHandler(
		ctx,
		service,
		zip.WithStore[composev1.ComposeSpec, composev1.ComposeStatus](embeddedStore, zip.StoreRehydrationSpecNil),
	)
}

func refreshRunningServices(ctx context.Context, embeddedProject *composev1.Compose) error {
	project, err := NewProjectFromComposeFile(ctx, embeddedProject.Spec.Workdir, embeddedProject.Spec.Composefile)
	if err != nil {
		return err
	}

	if err := project.Validate(ctx); err != nil {
		return err
	}

	controller, err := mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	if err != nil {
		return err
	}

	machines, err := controller.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return err
	}

	// We want to send warnings in two cases:
	// 1. Orphaned machines (machines that are no longer part of the project but are still running)
	// 2. Name collisions (machines that are part of the project and running, but not linked to the project)

	runningMachines := []metav1.ObjectMeta{}

	// Orphaned machines
	for _, machine := range embeddedProject.Status.Machines {
		isService := false
		for _, service := range project.Services {
			if service.Name == machine.Name {
				isService = true
				break
			}
		}

		for _, m := range machines.Items {
			if m.Name == machine.Name && m.Status.State == machineapi.MachineStateRunning {
				runningMachines = append(runningMachines, machine)
				if !isService {
					log.G(ctx).WithField("machine", machine.Name).Warn("found orphan machine")
				}
			}
		}
	}

	// Name collisions
	for _, m := range machines.Items {
		if m.Status.State != machineapi.MachineStateRunning {
			continue
		}
		isService := false
		for _, service := range project.Services {
			if service.Name == m.Name {
				isService = true
				break
			}
		}

		if !isService {
			continue
		}

		belongToProject := false
		for _, machine := range runningMachines {
			if machine.Name == m.Name {
				belongToProject = true
				break
			}
		}

		if isService && !belongToProject {
			log.G(ctx).WithField("machine", m.Name).Warn("machine already running but does not belong to project")
		}
	}

	embeddedProject.Status.Machines = runningMachines

	return nil
}

// Create implements kraftkit.sh/api/compose/v1.ComposeService
func (p *v1ComposeProject) Create(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	return project, nil
}

// Delete implements kraftkit.sh/api/compose/v1.ComposeService
func (p *v1ComposeProject) Delete(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	if err := refreshRunningServices(ctx, project); err != nil {
		return project, err
	}

	return nil, nil
}

// List implements kraftkit.sh/api/compose/v1.ComposeService
func (p *v1ComposeProject) List(ctx context.Context, projects *composev1.ComposeList) (*composev1.ComposeList, error) {
	for i := range projects.Items {
		if err := refreshRunningServices(ctx, &projects.Items[i]); err != nil {
			return projects, err
		}
	}

	return projects, nil
}

// Get implements kraftkit.sh/api/compose/v1.ComposeService
func (p *v1ComposeProject) Get(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	if err := refreshRunningServices(ctx, project); err != nil {
		return project, err
	}

	return project, nil
}

// Update implements kraftkit.sh/api/compose/v1.ComposeService
func (p *v1ComposeProject) Update(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	return project, nil
}

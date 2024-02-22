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
	"kraftkit.sh/machine/network"
	"kraftkit.sh/store"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	mplatform "kraftkit.sh/machine/platform"
)

type v1Compose struct{}

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

	service := &v1Compose{}

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

func refreshExistingNetworks(ctx context.Context, embeddedProject *composev1.Compose) error {
	project, err := NewProjectFromComposeFile(ctx, embeddedProject.Spec.Workdir, embeddedProject.Spec.Composefile)
	if err != nil {
		return err
	}

	if err := project.Validate(ctx); err != nil {
		return err
	}

	controller, err := network.NewNetworkV1alpha1ServiceIterator(ctx)
	if err != nil {
		return err
	}

	existingNetworks := []metav1.ObjectMeta{}

	allNetworks, err := controller.List(ctx, &networkapi.NetworkList{})
	if err != nil {
		return err
	}

	for i, network := range embeddedProject.Status.Networks {
		if network.Name[0] == '_' {
			network.Name = project.Name + network.Name
			embeddedProject.Status.Networks[i] = network
		}
		for _, n := range allNetworks.Items {
			if n.Name == network.Name {
				existingNetworks = append(existingNetworks, n.ObjectMeta)
				break
			}
		}
	}

	embeddedProject.Status.Networks = existingNetworks

	for _, network := range project.Networks {
		belongToProject := false
		for _, existingNetwork := range existingNetworks {
			if network.Name == existingNetwork.Name {
				belongToProject = true
				break
			}
		}

		if belongToProject {
			continue
		}

		for _, n := range allNetworks.Items {
			if n.Name == network.Name {
				log.G(ctx).WithField("network", network.Name).Warn("network already exists but does not belong to project")
				break
			}
		}
	}

	embeddedProject.Status.Networks = existingNetworks

	return nil
}

// Create implements kraftkit.sh/api/compose/v1.ComposeService
func (p *v1Compose) Create(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	return project, nil
}

// Delete implements kraftkit.sh/api/compose/v1.ComposeService
func (p *v1Compose) Delete(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	if err := refreshRunningServices(ctx, project); err != nil {
		return project, err
	}

	if err := refreshExistingNetworks(ctx, project); err != nil {
		return project, err
	}

	return nil, nil
}

// List implements kraftkit.sh/api/compose/v1.ComposeService
func (p *v1Compose) List(ctx context.Context, projects *composev1.ComposeList) (*composev1.ComposeList, error) {
	for i := range projects.Items {
		if err := refreshRunningServices(ctx, &projects.Items[i]); err != nil {
			return projects, err
		}

		if err := refreshExistingNetworks(ctx, &projects.Items[i]); err != nil {
			return projects, err
		}
	}

	return projects, nil
}

// Get implements kraftkit.sh/api/compose/v1.ComposeService
func (p *v1Compose) Get(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	if err := refreshRunningServices(ctx, project); err != nil {
		return project, err
	}

	if err := refreshExistingNetworks(ctx, project); err != nil {
		return project, err
	}

	return project, nil
}

// Update implements kraftkit.sh/api/compose/v1.ComposeService
func (p *v1Compose) Update(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	return project, nil
}

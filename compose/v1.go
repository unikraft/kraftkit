// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package compose

import (
	"context"
	"fmt"
	"path/filepath"

	zip "api.zip"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	composev1 "kraftkit.sh/api/compose/v1"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/network"
	"kraftkit.sh/machine/volume"
	"kraftkit.sh/store"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	volumeapi "kraftkit.sh/api/volume/v1alpha1"
	mplatform "kraftkit.sh/machine/platform"
)

type v1Compose struct {
	machineController machineapi.MachineService
	networkController networkapi.NetworkService
	volumeController  volumeapi.VolumeService
}

var ErrInvalidComposefile = fmt.Errorf("the Composefile for the project is either missing or invalid")

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

	service.machineController, err = mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	if err != nil {
		return nil, err
	}

	service.networkController, err = network.NewNetworkV1alpha1ServiceIterator(ctx)
	if err != nil {
		return nil, err
	}

	service.volumeController, err = volume.NewVolumeV1alpha1ServiceIterator(ctx)
	if err != nil {
		return nil, err
	}

	return composev1.NewComposeServiceHandler(
		ctx,
		service,
		zip.WithStore[composev1.ComposeSpec, composev1.ComposeStatus](embeddedStore, zip.StoreRehydrationSpecNil),
	)
}

func (v1 *v1Compose) refreshRunningServices(ctx context.Context, embeddedProject *composev1.Compose, project *Project) error {
	machines, err := v1.machineController.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return err
	}

	// We want to send warnings in two cases:
	// 1. Orphaned machines (machines that are no longer part of the project but are still running)
	// 2. Name collisions (machines that are part of the project and running, but not linked to the project)

	runningMachines := []metav1.ObjectMeta{}

	// Orphaned machines
	orphanMachines := []string{}
	for _, machine := range embeddedProject.Status.Machines {
		isService := false
		for _, service := range project.Services {
			if service.ContainerName == machine.Name {
				isService = true
				break
			}
		}

		for _, m := range machines.Items {
			if m.Name == machine.Name {
				runningMachines = append(runningMachines, machine)
				if !isService && m.Status.State == machineapi.MachineStateRunning {
					orphanMachines = append(orphanMachines, machine.Name)
				}
			}
		}
	}

	if len(orphanMachines) > 0 {
		log.G(ctx).WithField("machines", orphanMachines).
			Warn("found orphan machines for this project. You can run this command with the --remove-orphans flag to clean it up")
	}

	// Name collisions
	for _, m := range machines.Items {
		if m.Status.State != machineapi.MachineStateRunning {
			continue
		}
		isService := false
		for _, service := range project.Services {
			if service.ContainerName == m.Name {
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

func (v1 *v1Compose) refreshExistingNetworks(ctx context.Context, embeddedProject *composev1.Compose, project *Project) error {
	existingNetworks := []metav1.ObjectMeta{}

	allNetworks, err := v1.networkController.List(ctx, &networkapi.NetworkList{})
	if err != nil {
		return err
	}

	for _, network := range embeddedProject.Status.Networks {
		for _, n := range allNetworks.Items {
			if n.Name == network.Name {
				existingNetworks = append(existingNetworks, n.ObjectMeta)
				break
			}
		}
	}

	embeddedProject.Status.Networks = existingNetworks

	for _, network := range project.Networks {
		if network.External {
			continue
		}
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

	return nil
}

func (v1 *v1Compose) refreshExistingVolumes(ctx context.Context, embeddedProject *composev1.Compose, project *Project) error {
	existingVolumes := []metav1.ObjectMeta{}

	allVolumes, err := v1.volumeController.List(ctx, &volumeapi.VolumeList{})
	if err != nil {
		return err
	}

	for _, volume := range embeddedProject.Status.Volumes {
		for _, v := range allVolumes.Items {
			if v.Name == volume.Name {
				existingVolumes = append(existingVolumes, v.ObjectMeta)
				break
			}
		}
	}

	embeddedProject.Status.Volumes = existingVolumes

	for _, volume := range project.Volumes {
		if volume.External {
			continue
		}
		belongToProject := false
		for _, existingVolume := range existingVolumes {
			if volume.Name == existingVolume.Name {
				belongToProject = true
				break
			}
		}

		if belongToProject {
			continue
		}

		for _, v := range allVolumes.Items {
			if v.Name == volume.Name {
				log.G(ctx).WithField("volume", volume.Name).Warn("volume already exists but does not belong to project")
				break
			}
		}
	}

	return nil
}

func (v1 *v1Compose) refreshStatus(ctx context.Context, embeddedProject *composev1.Compose) error {
	project, err := NewProjectFromComposeFile(ctx, embeddedProject.Spec.Workdir, embeddedProject.Spec.Composefile)
	if err != nil {
		return ErrInvalidComposefile
	}

	if err := project.Validate(ctx); err != nil {
		return ErrInvalidComposefile
	}

	if err := v1.refreshRunningServices(ctx, embeddedProject, project); err != nil {
		return err
	}

	if err := v1.refreshExistingNetworks(ctx, embeddedProject, project); err != nil {
		return err
	}

	if err := v1.refreshExistingVolumes(ctx, embeddedProject, project); err != nil {
		return err
	}

	return nil
}

// Create implements kraftkit.sh/api/compose/v1.ComposeService
func (v1 *v1Compose) Create(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	return project, nil
}

// Delete implements kraftkit.sh/api/compose/v1.ComposeService
func (v1 *v1Compose) Delete(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	if err := v1.refreshStatus(ctx, project); err != nil {
		return project, err
	}

	return nil, nil
}

// List implements kraftkit.sh/api/compose/v1.ComposeService
func (v1 *v1Compose) List(ctx context.Context, projects *composev1.ComposeList) (*composev1.ComposeList, error) {
	validItems := []composev1.Compose{}
	for i := range projects.Items {
		if err := v1.refreshStatus(ctx, &projects.Items[i]); err != nil {
			if err == ErrInvalidComposefile {
				continue
			}
			return projects, err
		}

		validItems = append(validItems, projects.Items[i])
	}

	projects.Items = validItems
	return projects, nil
}

// Get implements kraftkit.sh/api/compose/v1.ComposeService
func (v1 *v1Compose) Get(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	if err := v1.refreshStatus(ctx, project); err != nil {
		// If there is no such Composefile, remove the project
		if err == ErrInvalidComposefile {
			return nil, nil
		}

		return project, err
	}

	return project, nil
}

// Update implements kraftkit.sh/api/compose/v1.ComposeService
func (v1 *v1Compose) Update(ctx context.Context, project *composev1.Compose) (*composev1.Compose, error) {
	return project, nil
}

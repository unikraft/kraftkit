// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package platform

import (
	"context"
	"fmt"

	zip "api.zip"
	"github.com/acorn-io/baaah/pkg/merr"

	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
)

type machineV1alpha1ServiceIterator struct {
	strategies map[Platform]machinev1alpha1.MachineService
}

// NewMachineV1alpha1ServiceIterator returns a
// machinev1alpha1.MachineService-compatible implementation which iterates over
// each supported host platform and calls the representing method.  This is
// useful in circumstances where the platform is not supplied.  The first
// platform strategy to succeed is returned in all circumstances.
func NewMachineV1alpha1ServiceIterator(ctx context.Context) (machinev1alpha1.MachineService, error) {
	var err error
	iterator := machineV1alpha1ServiceIterator{
		strategies: map[Platform]machinev1alpha1.MachineService{},
	}

	for platform, strategy := range hostSupportedStrategies() {
		iterator.strategies[platform], err = strategy.NewMachineV1alpha1(ctx)
		if err != nil {
			return nil, err
		}
	}

	return &iterator, nil
}

// Create implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (iterator *machineV1alpha1ServiceIterator) Create(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Create(ctx, machine)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return machine, fmt.Errorf("all iterated platforms failed: %w", merr.NewErrors(errs...))
}

// Start implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (iterator *machineV1alpha1ServiceIterator) Start(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Start(ctx, machine)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return machine, fmt.Errorf("all iterated platforms failed: %w", merr.NewErrors(errs...))
}

// Pause implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (iterator *machineV1alpha1ServiceIterator) Pause(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Pause(ctx, machine)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return machine, fmt.Errorf("all iterated platforms failed: %w", merr.NewErrors(errs...))
}

// Stop implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (iterator *machineV1alpha1ServiceIterator) Stop(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Stop(ctx, machine)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return machine, fmt.Errorf("all iterated platforms failed: %w", merr.NewErrors(errs...))
}

// Update implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (iterator *machineV1alpha1ServiceIterator) Update(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Update(ctx, machine)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return machine, fmt.Errorf("all iterated platforms failed: %w", merr.NewErrors(errs...))
}

// Delete implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (iterator *machineV1alpha1ServiceIterator) Delete(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Delete(ctx, machine)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return machine, fmt.Errorf("all iterated platforms failed: %w", merr.NewErrors(errs...))
}

// Get implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (iterator *machineV1alpha1ServiceIterator) Get(ctx context.Context, machine *machinev1alpha1.Machine) (*machinev1alpha1.Machine, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Get(ctx, machine)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return machine, fmt.Errorf("all iterated platforms failed: %w", merr.NewErrors(errs...))
}

// List implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (iterator *machineV1alpha1ServiceIterator) List(ctx context.Context, cached *machinev1alpha1.MachineList) (*machinev1alpha1.MachineList, error) {
	found := []zip.Object[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus]{}

	for _, strategy := range iterator.strategies {
		ret, err := strategy.List(ctx, &machinev1alpha1.MachineList{})
		if err != nil {
			continue
		}

		found = append(found, ret.Items...)
	}

	cached.Items = found

	return cached, nil
}

// Watch implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (iterator *machineV1alpha1ServiceIterator) Watch(ctx context.Context, machine *machinev1alpha1.Machine) (chan *machinev1alpha1.Machine, chan error, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		eventChan, errChan, err := strategy.Watch(ctx, machine)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return eventChan, errChan, nil
	}

	return nil, nil, fmt.Errorf("all iterated platforms failed: %w", merr.NewErrors(errs...))
}

// Logs implements kraftkit.sh/api/machine/v1alpha1.MachineService
func (iterator *machineV1alpha1ServiceIterator) Logs(ctx context.Context, machine *machinev1alpha1.Machine) (chan string, chan error, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		logChan, errChan, err := strategy.Logs(ctx, machine)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return logChan, errChan, nil
	}

	return nil, nil, fmt.Errorf("all iterated platforms failed: %w", merr.NewErrors(errs...))
}

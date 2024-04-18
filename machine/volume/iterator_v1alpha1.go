// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package volume

import (
	"context"
	"fmt"

	zip "api.zip"
	"github.com/acorn-io/baaah/pkg/merr"
	volumev1alpha1 "kraftkit.sh/api/volume/v1alpha1"
)

type volumeV1alpha1ServiceIterator struct {
	strategies map[string]volumev1alpha1.VolumeService
}

// NewVolumeV1alpha1ServiceIterator returns a
// volumev1alpha1.VolumeService -compatible implementation which iterates over
// each supported volume driver and calls the representing method.  This is
// useful in circumstances where the driver is not supplied.  The first volume
// driver to succeed is returned in all circumstances.
func NewVolumeV1alpha1ServiceIterator(ctx context.Context) (volumev1alpha1.VolumeService, error) {
	var err error
	iterator := volumeV1alpha1ServiceIterator{
		strategies: map[string]volumev1alpha1.VolumeService{},
	}

	for driver, strategy := range hostSupportedStrategies() {
		iterator.strategies[driver], err = strategy.NewVolumeV1alpha1(ctx)
		if err != nil {
			return nil, err
		}
	}

	return &iterator, nil
}

// Create implements kraftkit.sh/api/volume/v1alpha1.Create
func (iterator *volumeV1alpha1ServiceIterator) Create(ctx context.Context, volume *volumev1alpha1.Volume) (*volumev1alpha1.Volume, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Create(ctx, volume)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return volume, fmt.Errorf("all iterated drivers failed: %w", merr.NewErrors(errs...))
}

// Update implements kraftkit.sh/api/volume/v1alpha1.Update.
func (iterator *volumeV1alpha1ServiceIterator) Update(ctx context.Context, volume *volumev1alpha1.Volume) (*volumev1alpha1.Volume, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Update(ctx, volume)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return volume, fmt.Errorf("all iterated drivers failed: %w", merr.NewErrors(errs...))
}

// Delete implements kraftkit.sh/api/volume/v1alpha1.Delete
func (iterator *volumeV1alpha1ServiceIterator) Delete(ctx context.Context, volume *volumev1alpha1.Volume) (*volumev1alpha1.Volume, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Delete(ctx, volume)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return volume, fmt.Errorf("all iterated drivers failed: %w", merr.NewErrors(errs...))
}

// Get implements kraftkit.sh/api/volume/v1alpha1.Get
func (iterator *volumeV1alpha1ServiceIterator) Get(ctx context.Context, volume *volumev1alpha1.Volume) (*volumev1alpha1.Volume, error) {
	var errs []error

	for _, strategy := range iterator.strategies {
		ret, err := strategy.Get(ctx, volume)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		return ret, nil
	}

	return volume, fmt.Errorf("all iterated drivers failed: %w", merr.NewErrors(errs...))
}

// List implements kraftkit.sh/api/volume/v1alpha1.List
func (iterator *volumeV1alpha1ServiceIterator) List(ctx context.Context, cached *volumev1alpha1.VolumeList) (*volumev1alpha1.VolumeList, error) {
	found := []zip.Object[volumev1alpha1.VolumeSpec, volumev1alpha1.VolumeStatus]{}

	for _, strategy := range iterator.strategies {
		ret, err := strategy.List(ctx, &volumev1alpha1.VolumeList{})
		if err != nil {
			continue
		}

		found = append(found, ret.Items...)
	}

	cached.Items = found

	return cached, nil
}

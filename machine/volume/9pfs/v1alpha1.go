// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ninepfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/util/uuid"

	volumev1alpha1 "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
)

type v1alpha1Volume struct{}

func NewVolumeServiceV1alpha1(ctx context.Context, opts ...any) (volumev1alpha1.VolumeService, error) {
	return &v1alpha1Volume{}, nil
}

// Create implements kraftkit.sh/api/volume/v1alpha1.Create
func (*v1alpha1Volume) Create(ctx context.Context, volume *volumev1alpha1.Volume) (*volumev1alpha1.Volume, error) {
	var err error

	if len(volume.Spec.Driver) == 0 {
		volume.Spec.Driver = "9pfs"
	} else if volume.Spec.Driver != "9pfs" {
		return volume, fmt.Errorf("cannot use 9pfs driver when driver set to %s", volume.Spec.Driver)
	}

	if volume.ObjectMeta.UID == "" {
		volume.ObjectMeta.UID = uuid.NewUUID()
	}

	if volume.ObjectMeta.Name == "" {
		volume.ObjectMeta.Name = string(volume.ObjectMeta.UID)
	}

	if len(volume.Spec.Source) == 0 {
		// If no Source is specified, create a new volume entry in the runtime store
		log.G(ctx).Debugf("creating new volume entry in the runtime store %s", volume.ObjectMeta.UID)
		volume.Spec.Source = filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, "volumes", string(volume.ObjectMeta.UID))
		volume.Spec.Managed = true
	} else {
		volume.Spec.Managed = false
	}

	volume.Spec.Source, err = filepath.Abs(volume.Spec.Source)
	if err != nil {
		return volume, fmt.Errorf("cannot get absolute path for volume source: %w", err)
	}

	// Create the volume directory if it does not exist
	if err := os.MkdirAll(volume.Spec.Source, 0o755); err != nil {
		return volume, fmt.Errorf("cannot create volume directory: %w", err)
	}

	fileInfo, err := os.Stat(volume.Spec.Source)
	if err != nil {
		return volume, fmt.Errorf("cannot stat volume directory: %w", err)
	}

	if !fileInfo.IsDir() {
		return volume, fmt.Errorf("volume source is not a directory: %s", volume.Spec.Source)
	}

	volume.Status.State = volumev1alpha1.VolumeStatePending

	return volume, nil
}

// Delete implements kraftkit.sh/api/volume/v1alpha1.Delete
func (*v1alpha1Volume) Delete(_ context.Context, volume *volumev1alpha1.Volume) (*volumev1alpha1.Volume, error) {
	if len(volume.Spec.Driver) == 0 || volume.Spec.Driver != "9pfs" {
		return nil, nil
	}

	if len(volume.Spec.Source) == 0 {
		return nil, nil
	}

	if volume.Status.State == volumev1alpha1.VolumeStateBound {
		return volume, fmt.Errorf("cannot delete volume in state %s", volume.Status.State)
	}

	if volume.Spec.Managed {
		if err := os.RemoveAll(volume.Spec.Source); err != nil {
			return volume, fmt.Errorf("cannot remove volume directory: %w", err)
		}
	}

	return nil, nil
}

// Get implements kraftkit.sh/api/volume/v1alpha1.Get
func (*v1alpha1Volume) Get(_ context.Context, volume *volumev1alpha1.Volume) (*volumev1alpha1.Volume, error) {
	if len(volume.Spec.Driver) == 0 || volume.Spec.Driver != "9pfs" {
		return nil, nil
	}

	if len(volume.Spec.Source) == 0 {
		return nil, nil
	}

	return volume, nil
}

// List implements kraftkit.sh/api/volume/v1alpha1.List
func (*v1alpha1Volume) List(_ context.Context, volumes *volumev1alpha1.VolumeList) (*volumev1alpha1.VolumeList, error) {
	return volumes, nil
}

// Update implements kraftkit.sh/api/volume/v1alpha1.Update
func (*v1alpha1Volume) Update(_ context.Context, volume *volumev1alpha1.Volume) (*volumev1alpha1.Volume, error) {
	return volume, nil
}

// Watch implements kraftkit.sh/api/volume/v1alpha1.Watch
func (*v1alpha1Volume) Watch(context.Context, *volumev1alpha1.Volume) (chan *volumev1alpha1.Volume, chan error, error) {
	panic("not implemented: kraftkit.sh/machine/volume/9pfs.v1alpha1Volume.Watch")
}

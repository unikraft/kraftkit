// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package docker

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/moby/moby/client"

	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

type dockerManager struct {
	registries []string
}

const DockerFormat pack.PackageFormat = "docker"

// NewDockerManager instantiates a new package manager based on local docker images.
func NewDockerManager(ctx context.Context, opts ...any) (packmanager.PackageManager, error) {
	manager := dockerManager{}

	for _, mopt := range opts {
		opt, ok := mopt.(dockerManagerOption)
		if !ok {
			return nil, fmt.Errorf("cannot cast docker Manager option")
		}

		if err := opt(ctx, &manager); err != nil {
			return nil, err
		}
	}

	return &manager, nil
}

// Update implements packmanager.PackageManager
func (manager *dockerManager) Update(ctx context.Context) error {
	return nil
}

// Pack implements packmanager.PackageManager
func (manager *dockerManager) Pack(ctx context.Context, entity component.Component, opts ...packmanager.PackOption) ([]pack.Package, error) {
	_, ok := entity.(target.Target)
	if !ok {
		return nil, fmt.Errorf("entity is not Unikraft target")
	}

	return nil, fmt.Errorf("not implemented: docker.manager.Pack")
}

// Unpack implements packmanager.PackageManager
func (manager *dockerManager) Unpack(ctx context.Context, entity pack.Package, opts ...packmanager.UnpackOption) ([]component.Component, error) {
	return nil, fmt.Errorf("not implemented: docker.manager.Unpack")
}

// Catalog implements packmanager.PackageManager
func (manager *dockerManager) Catalog(ctx context.Context, qopts ...packmanager.QueryOption) ([]pack.Package, error) {
	// isolate the name of the image
	query := packmanager.NewQuery(qopts...)

	// Connect to Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	defer dockerClient.Close()

	// Check if image is in local Docker storage
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, err
	}

	for _, i := range images {
		for _, t := range i.RepoTags {
			if t == query.Name() {
				// TODO(jake-ciolek): right now we default to qemu/x86_64
				// figure out what's the best way of passing this in the image.
				arch, err := arch.NewArchitectureFromSchema("x86_64")
				if err != nil {
					return nil, err
				}
				plat, err := plat.NewPlatformFromOptions(plat.WithName("qemu"))
				if err != nil {
					return nil, err
				}
				return []pack.Package{&DockerImage{
					ID:   i.ID,
					arch: arch,
					plat: plat,
				}}, nil
			}
		}
	}

	return nil, errors.New("unable to find the requested image in local docker storage")
}

// SetSources implements packmanager.PackageManager
func (manager *dockerManager) SetSources(_ context.Context, sources ...string) error {
	manager.registries = sources
	return nil
}

// AddSource implements packmanager.PackageManager
func (manager *dockerManager) AddSource(ctx context.Context, source string) error {
	if manager.registries == nil {
		manager.registries = make([]string, 0)
	}

	manager.registries = append(manager.registries, source)

	return nil
}

// RemoveSource implements packmanager.PackageManager
func (manager *dockerManager) RemoveSource(ctx context.Context, source string) error {
	for i, needle := range manager.registries {
		if needle == source {
			ret := make([]string, 0)
			ret = append(ret, manager.registries[:i]...)
			manager.registries = append(ret, manager.registries[i+1:]...)
			break
		}
	}

	return nil
}

// IsCompatible implements packmanager.PackageManager
func (manager *dockerManager) IsCompatible(ctx context.Context, source string, qopts ...packmanager.QueryOption) (packmanager.PackageManager, bool, error) {
	// Check if an image with given name exists in local docker image storage

	// Connect to Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, false, err
	}
	defer dockerClient.Close()

	// Check if image is in local Docker storage
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return nil, false, err
	}

	for _, i := range images {
		for _, t := range i.RepoTags {
			if t == source {
				return manager, true, nil
			}
		}
	}

	return nil, false, nil
}

// From implements packmanager.PackageManager
func (manager *dockerManager) From(pack.PackageFormat) (packmanager.PackageManager, error) {
	return nil, fmt.Errorf("not possible: docker.manager.From")
}

// Format implements packmanager.PackageManager
func (manager *dockerManager) Format() pack.PackageFormat {
	return DockerFormat
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package docker

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/moby/moby/client"

	"kraftkit.sh/initrd"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/oci"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
)

type DockerImage struct {
	ID  string
	ref name.Reference

	// Embedded attributes which represent target.Target
	arch    arch.Architecture
	plat    plat.Platform
	kconfig kconfig.KeyValueMap
	kernel  string
	initrd  *initrd.InitrdConfig
	command []string
}

// Type implements unikraft.Nameable
func (dockerImage *DockerImage) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

// Name implements unikraft.Nameable
func (dockerImage *DockerImage) Name() string {
	return dockerImage.ref.Context().Name()
}

// Version implements unikraft.Nameable
func (dockerImage *DockerImage) Version() string {
	return dockerImage.ref.Identifier()
}

// Metadata implements pack.Package
func (dockerImage *DockerImage) Metadata() any {
	return nil
}

// Push implements pack.Package
func (dockerImage *DockerImage) Push(ctx context.Context, opts ...pack.PushOption) error {
	return errors.New("not implemented")
}

func (dockerImage *DockerImage) Pull(ctx context.Context, opts ...pack.PullOption) error {
	targetImageID := dockerImage.ID

	// Setup the pull options
	popts, err := pack.NewPullOptions(opts...)
	if err != nil {
		return err
	}

	// Connect to Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	// Check if image is in local Docker storage
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return err
	}

	imageFound := false
	for _, img := range images {
		if img.ID == targetImageID {
			imageFound = true
			break
		}
	}

	if !imageFound {
		return errors.New("target image not found in local Docker storage")
	}

	if len(popts.Workdir()) > 0 {
		// Unpack the image to the provided working directory
		reader, err := dockerClient.ImageSave(context.Background(), []string{targetImageID})
		if err != nil {
			return err
		}
		defer reader.Close()

		// Create a unique path to save the image tarball in the temp directory
		file, err := os.CreateTemp(os.TempDir(), "docker_image_*.tar")
		if err != nil {
			return err
		}
		savePath := file.Name()
		defer file.Close()

		// Copy the image data from reader to file
		_, err = io.Copy(file, reader)
		if err != nil {
			return err
		}

		if err := unpackTar(savePath, popts.Workdir()); err != nil {
			return err
		}

		if err := mergeLayersFromManifest(popts.Workdir()); err != nil {
			return err
		}

		// Set the kernel path
		dockerImage.kernel = filepath.Join(popts.Workdir(), oci.WellKnownKernelPath)
	}

	return nil
}

type ImageManifest struct {
	Layers []string `json:"Layers"`
}

func mergeLayersFromManifest(workdir string) error {
	// Read manifest.json to get the order of layers
	manifestData, err := os.ReadFile(filepath.Join(workdir, "manifest.json"))
	if err != nil {
		return err
	}

	var manifests []ImageManifest
	if err := json.Unmarshal(manifestData, &manifests); err != nil {
		return err
	}

	for _, layerPath := range manifests[0].Layers {
		layerTarPath := filepath.Join(workdir, layerPath)
		if err := unpackTar(layerTarPath, workdir); err != nil {
			return err
		}
	}
	return nil
}

func unpackTar(src, dest string) error {
	tarFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	tr := tar.NewReader(tarFile)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_RDWR, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

// Pull implements pack.Package
func (dockerImage *DockerImage) Format() pack.PackageFormat {
	return DockerFormat
}

// Source implements unikraft.component.Component
func (dockerImage *DockerImage) Source() string {
	return ""
}

// Path implements unikraft.component.Component
func (dockerImage *DockerImage) Path() string {
	return ""
}

// KConfigTree implements unikraft.component.Component
func (dockerImage DockerImage) KConfigTree(context.Context, ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, fmt.Errorf("not implemented: docker.dockerImage.KConfigTree")
}

// KConfig implements unikraft.component.Component
func (dockerImage DockerImage) KConfig() kconfig.KeyValueMap {
	return dockerImage.kconfig
}

// PrintInfo implements unikraft.component.Component
func (dockerImage DockerImage) PrintInfo(context.Context) string {
	return "not implemented: docker.dockerImage.PrintInfo"
}

// Architecture implements unikraft.target.Target
func (dockerImage DockerImage) Architecture() arch.Architecture {
	return dockerImage.arch
}

// Platform implements unikraft.target.Target
func (dockerImage DockerImage) Platform() plat.Platform {
	return dockerImage.plat
}

// Kernel implements unikraft.target.Target
func (dockerImage DockerImage) Kernel() string {
	return dockerImage.kernel
}

// KernelDbg implements unikraft.target.Target
func (dockerImage DockerImage) KernelDbg() string {
	return dockerImage.kernel
}

// Initrd implements unikraft.target.Target
func (dockerImage DockerImage) Initrd() *initrd.InitrdConfig {
	return dockerImage.initrd
}

// Command implements unikraft.target.Target
func (dockerImage DockerImage) Command() []string {
	if len(dockerImage.command) == 0 {
		return []string{"--"}
	}

	return dockerImage.command
}

// ConfigFilename implements unikraft.target.Target
func (dockerImage DockerImage) ConfigFilename() string {
	return ""
}

// MarshalYAML implements unikraft.target.Target (yaml.Marshaler)
func (dockerImage *DockerImage) MarshalYAML() (interface{}, error) {
	if dockerImage == nil {
		return nil, nil
	}

	return map[string]interface{}{
		"architecture": dockerImage.arch.Name(),
		"platform":     dockerImage.plat.Name(),
	}, nil
}

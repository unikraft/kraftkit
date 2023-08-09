// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/google/go-containerregistry/pkg/name"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"

	"kraftkit.sh/oci/handler"
)

type ImageIndex struct {
	workdir  string
	autoSave bool

	handle handler.Handler

	index     ocispec.Index
	indexDesc ocispec.Descriptor

	manifests   []ocispec.Manifest
	images      []ocispec.Image
	annotations map[string]string
}

func NewImageIndex(_ context.Context, handle handler.Handler, opts ...ImageIndexOption) (*ImageIndex, error) {
	if handle == nil {
		return nil, fmt.Errorf("cannot use `NewImageIndex` without handler")
	}

	index := ImageIndex{
		handle:   handle,
		autoSave: true,
	}

	for _, opt := range opts {
		if err := opt(&index); err != nil {
			return nil, err
		}
	}

	return &index, nil
}

// AddManifest adds a manifest directly to the index and returns the resulting
// descriptor
func (index *ImageIndex) AddManifest(ctx context.Context, manifest *ocispec.Manifest, image *ocispec.Image) (ocispec.Descriptor, error) {
	manifestJson, err := json.Marshal(manifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	descriptor := content.NewDescriptorFromBytes(
		ocispec.MediaTypeImageManifest,
		manifestJson,
	)

	descriptor.Platform = &ocispec.Platform{
		Architecture: image.Architecture,
		OS:           image.OS,
		OSFeatures:   image.OSFeatures,
	}

	index.index.Manifests = append(index.index.Manifests, descriptor)

	return descriptor, nil
}

// SetAnnotation sets an anootation of the index with the provided key.
func (index *ImageIndex) SetAnnotation(_ context.Context, key, val string) {
	if index.annotations == nil {
		index.annotations = make(map[string]string)
	}

	index.annotations[key] = val
}

// Save the index.
func (index *ImageIndex) Save(ctx context.Context, source string, onProgress func(float64)) (ocispec.Descriptor, error) {
	ref, err := name.ParseReference(source,
		name.WithDefaultRegistry(DefaultRegistry),
	)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	// General annotations
	index.annotations[ocispec.AnnotationRefName] = ref.Context().String()
	index.annotations[ocispec.AnnotationRevision] = ref.Identifier()
	index.annotations[ocispec.AnnotationCreated] = time.Now().UTC().Format(time.RFC3339)

	// containerd compatibility annotations
	index.annotations[images.AnnotationImageName] = ref.String()

	// Add Manifests
	for idx, manifest := range index.manifests {
		_, err := index.AddManifest(ctx, &manifest, &index.images[idx])
		if err != nil {
			return ocispec.Descriptor{}, err
		}
	}

	index.index.Annotations = index.annotations
	index.index.SchemaVersion = 2

	indexJson, err := json.Marshal(index.index)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	index.indexDesc = content.NewDescriptorFromBytes(
		ocispec.MediaTypeImageIndex,
		indexJson,
	)
	index.indexDesc.ArtifactType = index.index.ArtifactType
	index.indexDesc.Size = int64(len(indexJson))

	// save the manifest digest
	if err := index.handle.SaveDigest(
		ctx,
		source,
		index.indexDesc,
		bytes.NewReader(indexJson),
		onProgress,
	); err != nil && !errors.Is(err, errdefs.ErrAlreadyExists) {
		return ocispec.Descriptor{}, fmt.Errorf("failed to push manifest: %w", err)
	}

	return index.indexDesc, nil
}

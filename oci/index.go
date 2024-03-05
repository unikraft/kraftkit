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
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"

	"kraftkit.sh/internal/version"
	"kraftkit.sh/oci/handler"
)

type Index struct {
	handle      handler.Handler
	index       *ocispec.Index
	desc        *ocispec.Descriptor
	manifests   []*Manifest
	annotations map[string]string
	saved       bool
}

// NewIndex instantiates a new index using the provided handler.
func NewIndex(_ context.Context, handle handler.Handler) (*Index, error) {
	if handle == nil {
		return nil, fmt.Errorf("cannot use `NewIndex` without handler")
	}

	index := Index{
		handle: handle,
		saved:  false,
	}

	return &index, nil
}

// NewIndexFromSpec instantiates a new index using the provided handler as well
// as a reference
func NewIndexFromSpec(ctx context.Context, handle handler.Handler, spec *ocispec.Index) (*Index, error) {
	index, err := NewIndex(ctx, handle)
	if err != nil {
		return nil, err
	}

	index.index = spec

	indexJson, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	indexDesc := content.NewDescriptorFromBytes(
		ocispec.MediaTypeImageIndex,
		indexJson,
	)

	index.desc = &indexDesc

	for _, desc := range spec.Manifests {
		manifest, err := NewManifestFromDigest(ctx, handle, desc.Digest)
		if err != nil {
			return nil, fmt.Errorf("could not instantiate manifest from structure: %w", err)
		}

		index.manifests = append(index.manifests, manifest)
	}

	return index, nil
}

// NewIndexFromRef instantiates a new index using the provided reference which
// is used by the handle to look up any local existing indexes.
func NewIndexFromRef(ctx context.Context, handle handler.Handler, ref string) (*Index, error) {
	spec, err := handle.ResolveIndex(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("cannot instantiate index from unknown reference '%s'", ref)
	}

	index, err := NewIndexFromSpec(ctx, handle, spec)
	if err != nil {
		return nil, err
	}

	index.saved = true

	return index, nil
}

// SetAnnotation sets an anootation of the image with the provided key.
func (index *Index) SetAnnotation(_ context.Context, key, val string) {
	if index.annotations == nil {
		index.annotations = make(map[string]string)
	}

	index.saved = false
	index.annotations[key] = val
}

// Annotations returns the map of annotations for the index.
func (index *Index) Annotations() map[string]string {
	return index.annotations
}

// IndexDesc returns the descriptor of the index.
func (index *Index) Descriptor() (*ocispec.Descriptor, error) {
	if index.desc != nil {
		return index.desc, nil
	}

	indexJson, err := json.Marshal(index.index)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	desc := content.NewDescriptorFromBytes(
		ocispec.MediaTypeImageIndex,
		indexJson,
	)
	desc.Annotations = index.Annotations()
	index.desc = &desc

	return &desc, nil
}

// AddManifest adds a manifest based an previously instantiated Manifest
// structure.
func (index *Index) AddManifest(_ context.Context, manifest *Manifest) error {
	if index.manifests == nil {
		index.manifests = []*Manifest{}
	}

	index.desc = nil
	index.saved = false

	for _, m := range index.manifests {
		if m.desc != nil && manifest.desc != nil && m.desc.Digest == manifest.desc.Digest {
			return nil
		}
	}

	index.manifests = append(index.manifests, manifest)

	return nil
}

// Save the index.
func (index *Index) Save(ctx context.Context, fullref string, onProgress func(float64)) (ocispec.Descriptor, error) {
	if index.saved {
		return *index.desc, nil
	}

	if index.desc != nil {
		if info, _ := index.handle.DigestInfo(ctx, index.desc.Digest); info != nil {
			return *index.desc, nil
		}
	}

	ref, err := name.ParseReference(fullref,
		name.WithDefaultRegistry(""),
		name.WithDefaultTag(DefaultTag),
	)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	// Pack the given blobs which generates an image manifest for the pack, and
	// pushes it to a content storage.
	if index.annotations == nil {
		index.annotations = make(map[string]string)
	}

	// General annotations
	index.annotations[ocispec.AnnotationRefName] = ref.Context().String()
	index.annotations[ocispec.AnnotationCreated] = time.Now().UTC().Format(time.RFC3339)
	index.annotations[AnnotationKraftKitVersion] = version.Version()

	// containerd compatibility annotations
	index.annotations[images.AnnotationImageName] = ref.String()

	manifestDescs := make([]ocispec.Descriptor, len(index.manifests))
	for i, manifest := range index.manifests {
		desc := manifest.desc
		if !manifest.saved {
			desc, err = manifest.Save(ctx, ref.Name(), nil)
			if err != nil {
				return ocispec.Descriptor{}, fmt.Errorf("could not save manifest: %w", err)
			}

			// Reset the index descriptor if a new manifest is added via local save
			index.index = nil
		}

		manifestDescs[i] = *desc
	}

	if index.index == nil {
		// Generate the final manifest
		index.index = &ocispec.Index{
			MediaType: ocispec.MediaTypeImageIndex,
			Versioned: specs.Versioned{
				SchemaVersion: 2,
			},
			Manifests:   manifestDescs,
			Annotations: index.annotations,
		}
		index.desc = nil
	}

	indexJson, err := json.Marshal(index.index)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if index.desc == nil {
		indexDesc := content.NewDescriptorFromBytes(
			ocispec.MediaTypeImageIndex,
			indexJson,
		)
		indexDesc.Annotations = index.index.Annotations
		index.desc = &indexDesc
	}

	// Remove the old index
	if err := index.handle.DeleteIndex(ctx, ref.Name(), false); err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to remove old index: %w", err)
	}

	// save the manifest digest
	if err := index.handle.SaveDescriptor(
		ctx,
		ref.Name(),
		*index.desc,
		bytes.NewReader(indexJson),
		onProgress,
	); err != nil && !errors.Is(err, errdefs.ErrAlreadyExists) {
		return ocispec.Descriptor{}, fmt.Errorf("failed to save manifest: %w", err)
	}

	index.saved = true

	return *index.desc, nil
}

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
	"os"
	"sync"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"oras.land/oras-go/v2/content"

	"kraftkit.sh/log"
	"kraftkit.sh/oci/handler"
)

type Manifest struct {
	handle handler.Handler

	config   *ocispec.Image
	manifest *ocispec.Manifest
	desc     *ocispec.Descriptor
	layers   []*Layer
	pushed   sync.Map // wraps map[digest.Digest]bool

	annotations map[string]string
}

// NewManifest instantiates a new image based in a handler and any provided
// options.
func NewManifest(ctx context.Context, handle handler.Handler) (*Manifest, error) {
	if handle == nil {
		return nil, fmt.Errorf("cannot use `NewImage` without handler")
	}

	manifest := Manifest{
		layers: make([]*Layer, 0),
		handle: handle,
		config: &ocispec.Image{
			Config: ocispec.ImageConfig{},
		},
	}

	return &manifest, nil
}

// NewManifestFromSpec instantiates a new image based in a handler and with
// the provided Manifest specification and options.
func NewManifestFromSpec(ctx context.Context, handle handler.Handler, spec ocispec.Manifest) (*Manifest, error) {
	manifest, err := NewManifest(ctx, handle)
	if err != nil {
		return nil, err
	}

	manifest.manifest = &spec

	return manifest, nil
}

// Layers returns the layers of this OCI image.
func (manifest *Manifest) Layers() []*Layer {
	return manifest.layers
}

// AddLayer adds a layer directly to the image and returns the resulting
// descriptor.
func (manifest *Manifest) AddLayer(ctx context.Context, layer *Layer) (ocispec.Descriptor, error) {
	if layer == nil {
		return ocispec.Descriptor{}, fmt.Errorf("cannot add empty layer")
	}

	log.G(ctx).WithFields(logrus.Fields{
		"dest":      layer.dst,
		"digest":    layer.blob.desc.Digest.String(),
		"mediaType": layer.blob.desc.MediaType,
	}).Trace("oci: layering")

	manifest.pushed.Store(layer.blob.desc.Digest, false)

	manifest.layers = append(manifest.layers, layer)

	return layer.blob.desc, nil
}

// AddBlob adds a blog to the manifest and returns the resulting descriptor.
func (manifest *Manifest) AddBlob(ctx context.Context, blob *Blob) (ocispec.Descriptor, error) {
	if exists, err := manifest.handle.DigestExists(ctx, blob.desc.Digest); err == nil && exists {
		log.G(ctx).WithFields(logrus.Fields{
			"mediaType": blob.desc.MediaType,
			"digest":    blob.desc.Digest.String(),
		}).Trace("oci: blob already exists")

		return blob.desc, err
	}

	log.G(ctx).WithFields(logrus.Fields{
		"mediaType": blob.desc.MediaType,
		"digest":    blob.desc.Digest.String(),
	}).Trace("oci: saving")

	fp, err := os.Open(blob.tmp)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	if err := manifest.handle.SaveDigest(ctx, "", blob.desc, fp, nil); err != nil {
		return ocispec.Descriptor{}, err
	}

	if err := fp.Close(); err != nil {
		return ocispec.Descriptor{}, err
	}

	if blob.removeAfterSave {
		if err := os.Remove(blob.tmp); err != nil {
			return ocispec.Descriptor{}, err
		}
	}

	return blob.desc, nil
}

// SetLabel sets a label of the manifest with the provided key.
func (manifest *Manifest) SetLabel(_ context.Context, key, val string) {
	if manifest.config.Config.Labels == nil {
		manifest.config.Config.Labels = make(map[string]string)
	}

	manifest.config.Config.Labels[key] = val
}

// SetAnnotation sets an anootation of the manifest with the provided key.
func (manifest *Manifest) SetAnnotation(_ context.Context, key, val string) {
	if manifest.annotations == nil {
		manifest.annotations = make(map[string]string)
	}

	manifest.annotations[key] = val
}

// SetArchitecture sets the architecture of the manifest.
func (manifest *Manifest) SetArchitecture(_ context.Context, architecture string) {
	manifest.config.Architecture = architecture
}

// SetOS sets the OS of the manifest.
func (manifest *Manifest) SetOS(_ context.Context, os string) {
	manifest.config.OS = os
}

// SetOSVersion sets the version of the OS of the manifest.
func (manifest *Manifest) SetOSVersion(_ context.Context, osversion string) {
	manifest.config.OSVersion = osversion
}

// SetOSFeature sets any OS features of the manifest.
func (manifest *Manifest) SetOSFeature(_ context.Context, feature ...string) {
	if manifest.config.OSFeatures == nil {
		manifest.config.OSFeatures = make([]string, 0)
	}

	manifest.config.OSFeatures = append(manifest.config.OSFeatures, feature...)
}

// Set the command of the manifest.
func (manifest *Manifest) SetCmd(_ context.Context, cmd []string) {
	manifest.config.Config.Cmd = cmd
}

// Save the manifest.
func (manifest *Manifest) Save(ctx context.Context, source string, onProgress func(float64)) (ocispec.Descriptor, error) {
	ref, err := name.ParseReference(source,
		name.WithDefaultRegistry(DefaultRegistry),
	)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	// Push any outstanding layers
	eg, egCtx := errgroup.WithContext(ctx)

	for i := range manifest.layers {
		eg.Go(func(i int) func() error {
			return func() error {
				pushed, exists := manifest.pushed.Load(manifest.layers[i].blob.desc.Digest)
				if exists && pushed.(bool) {
					return nil
				}

				if exists, _ = manifest.handle.DigestExists(ctx, manifest.layers[i].blob.desc.Digest); exists {
					return nil
				}

				if _, err := manifest.AddBlob(egCtx, manifest.layers[i].blob); err != nil {
					return fmt.Errorf("failed to push layer: %d: %v", i, err)
				}

				return nil
			}
		}(i))
	}
	if err := eg.Wait(); err != nil {
		return ocispec.Descriptor{}, err
	}

	// Copy the current set of layers, this will make up the manifest.
	var layers []ocispec.Descriptor
	var diffIds []digest.Digest

	for _, layer := range manifest.layers {
		layers = append(layers, layer.blob.desc)
		diffIds = append(diffIds, layer.blob.desc.Digest)
	}

	if len(diffIds) > 0 {
		manifest.config.RootFS = ocispec.RootFS{
			Type:    "layers",
			DiffIDs: diffIds,
		}
	}

	configJson, err := json.Marshal(manifest.config)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	configBlob, err := NewBlob(
		ctx,
		ocispec.MediaTypeImageConfig,
		configJson,
		WithBlobPlatform(&ocispec.Platform{
			Architecture: manifest.config.Architecture,
			OS:           manifest.config.OS,
			OSVersion:    manifest.config.OSVersion,
			OSFeatures:   manifest.config.OSFeatures,
		}),
	)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer os.Remove(configBlob.tmp)

	// We check if the blob already exists.  It is possible to have a duplicate
	// configuration already present if Save() is called repeatedly.
	if exists, _ := manifest.handle.DigestExists(ctx, configBlob.desc.Digest); !exists {
		log.G(ctx).WithFields(logrus.Fields{
			"digest": configBlob.desc.Digest,
		}).Trace("oci: saving config")

		if _, err := manifest.AddBlob(ctx, configBlob); err != nil {
			return ocispec.Descriptor{}, err
		}
	}

	log.G(ctx).Trace("oci: packing image manifest")

	// Pack the given blobs which generates an image manifest for the pack, and
	// pushes it to a content storage.
	if manifest.annotations == nil {
		manifest.annotations = make(map[string]string)
	}

	// General annotations
	manifest.annotations[ocispec.AnnotationRefName] = ref.Context().String()
	manifest.annotations[ocispec.AnnotationRevision] = ref.Identifier()
	manifest.annotations[ocispec.AnnotationCreated] = time.Now().UTC().Format(time.RFC3339)

	// containerd compatibility annotations
	manifest.annotations[images.AnnotationImageName] = ref.String()

	// Generate the final manifest
	manifest.manifest = &ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2, // historical value. does not pertain to OCI or docker version
		},
		Config:      configBlob.desc,
		MediaType:   ocispec.MediaTypeImageManifest,
		Layers:      layers,
		Annotations: manifest.annotations,
	}

	manifestJson, err := json.Marshal(manifest.manifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestDesc := content.NewDescriptorFromBytes(
		ocispec.MediaTypeImageManifest,
		manifestJson,
	)
	manifestDesc.ArtifactType = manifest.manifest.Config.MediaType
	manifestDesc.Annotations = manifest.manifest.Annotations

	// save the manifest digest
	if err := manifest.handle.SaveDigest(
		ctx,
		source,
		manifestDesc,
		bytes.NewReader(manifestJson),
		onProgress,
	); err != nil && !errors.Is(err, errdefs.ErrAlreadyExists) {
		return ocispec.Descriptor{}, fmt.Errorf("failed to push manifest: %w", err)
	}

	manifest.desc = &manifestDesc

	return manifestDesc, nil
}

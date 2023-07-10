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

type Image struct {
	workdir  string
	autoSave bool

	handle handler.Handler

	config       ocispec.Image
	manifest     ocispec.Manifest
	manifestDesc ocispec.Descriptor
	layers       []*Layer
	pushed       sync.Map // wraps map[digest.Digest]bool

	annotations map[string]string
}

// NewImage instantiates a new image based in a handler and any provided
// options.
func NewImage(_ context.Context, handle handler.Handler, opts ...ImageOption) (*Image, error) {
	if handle == nil {
		return nil, fmt.Errorf("cannot use `NewImage` without handler")
	}

	image := Image{
		layers:   make([]*Layer, 0),
		handle:   handle,
		autoSave: true,
		config: ocispec.Image{
			Config: ocispec.ImageConfig{},
		},
	}

	for _, opt := range opts {
		if err := opt(&image); err != nil {
			return nil, err
		}
	}

	return &image, nil
}

// NewImageFromManifestSpec instantiates a new image based in a handler and with
// the provided Manifest specification and options.
func NewImageFromManifestSpec(ctx context.Context, handle handler.Handler, manifest ocispec.Manifest, opts ...ImageOption) (*Image, error) {
	image, err := NewImage(ctx, handle, opts...)
	if err != nil {
		return nil, err
	}

	image.manifest = manifest

	// TODO(nderjung): This method could better populate the Image structure based
	// on parsing the of the manifest itself and probing for any embedded
	// configuration.
	// log.G(ctx).Warn("dangerously using `NewImageFromManifestSpec`")

	return image, nil
}

// Layers returns the layers of this OCI image.
func (image *Image) Layers() []*Layer {
	return image.layers
}

// AddLayer adds a layer directly to the image and returns the resulting
// descriptor.
func (image *Image) AddLayer(ctx context.Context, layer *Layer) (ocispec.Descriptor, error) {
	if layer == nil {
		return ocispec.Descriptor{}, fmt.Errorf("cannot add empty layer")
	}

	log.G(ctx).WithFields(logrus.Fields{
		"dest":      layer.dst,
		"digest":    layer.blob.desc.Digest.String(),
		"mediaType": layer.blob.desc.MediaType,
	}).Trace("oci: layering")

	if image.autoSave {
		if exists, _ := image.handle.DigestExists(ctx, layer.blob.desc.Digest); !exists {
			if _, err := image.AddBlob(ctx, layer.blob); err != nil {
				return ocispec.Descriptor{}, err
			}
		}

		image.pushed.Store(layer.blob.desc.Digest, true)

	} else {
		image.pushed.Store(layer.blob.desc.Digest, false)
	}

	image.layers = append(image.layers, layer)

	return layer.blob.desc, nil
}

// AddBlob adds a blog to the image and returns the resulting descriptor.
func (image *Image) AddBlob(ctx context.Context, blob *Blob) (ocispec.Descriptor, error) {
	if exists, err := image.handle.DigestExists(ctx, blob.desc.Digest); err == nil && exists {
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

	defer func() {
		closeErr := fp.Close()
		if err == nil {
			err = closeErr
		}
	}()

	if err := image.handle.SaveDigest(ctx, "", blob.desc, fp, nil); err != nil {
		return ocispec.Descriptor{}, err
	}

	if blob.removeAfterSave {
		if err := os.Remove(blob.tmp); err != nil {
			return ocispec.Descriptor{}, err
		}
	}

	return blob.desc, nil
}

// SetLabel sets a label of the image with the provided key.
func (image *Image) SetLabel(_ context.Context, key, val string) {
	if image.config.Config.Labels == nil {
		image.config.Config.Labels = make(map[string]string)
	}

	image.config.Config.Labels[key] = val
}

// SetAnnotation sets an anootation of the image with the provided key.
func (image *Image) SetAnnotation(_ context.Context, key, val string) {
	if image.annotations == nil {
		image.annotations = make(map[string]string)
	}

	image.annotations[key] = val
}

// SetArchitecture sets the architecture of the image.
func (image *Image) SetArchitecture(_ context.Context, architecture string) {
	image.config.Architecture = architecture
}

// SetOS sets the OS of the image.
func (image *Image) SetOS(_ context.Context, os string) {
	image.config.OS = os
}

// SetOSVersion sets the version of the OS of the image.
func (image *Image) SetOSVersion(_ context.Context, osversion string) {
	image.config.OSVersion = osversion
}

// SetOSFeature sets any OS features of the image.
func (image *Image) SetOSFeature(_ context.Context, feature ...string) {
	if image.config.OSFeatures == nil {
		image.config.OSFeatures = make([]string, 0)
	}

	image.config.OSFeatures = append(image.config.OSFeatures, feature...)
}

// Set the command of the image.
func (image *Image) SetCmd(_ context.Context, cmd []string) {
	image.config.Config.Cmd = cmd
}

// Save the image.
func (image *Image) Save(ctx context.Context, source string, onProgress func(float64)) (ocispec.Descriptor, error) {
	ref, err := name.ParseReference(source,
		name.WithDefaultRegistry(DefaultRegistry),
	)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	// Push any outstanding layers
	eg, egCtx := errgroup.WithContext(ctx)

	for i := range image.layers {
		eg.Go(func(i int) func() error {
			return func() error {
				pushed, exists := image.pushed.Load(image.layers[i].blob.desc.Digest)
				if exists && pushed.(bool) {
					return nil
				}

				if exists, _ = image.handle.DigestExists(ctx, image.layers[i].blob.desc.Digest); exists {
					return nil
				}

				if _, err := image.AddBlob(egCtx, image.layers[i].blob); err != nil {
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

	for _, layer := range image.layers {
		layers = append(layers, layer.blob.desc)
		diffIds = append(diffIds, layer.blob.desc.Digest)
	}

	if len(diffIds) > 0 {
		image.config.RootFS = ocispec.RootFS{
			Type:    "layers",
			DiffIDs: diffIds,
		}
	}

	configJson, err := json.Marshal(image.config)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	configBlob, err := NewBlob(
		ctx,
		ocispec.MediaTypeImageConfig,
		configJson,
		WithBlobPlatform(&ocispec.Platform{
			Architecture: image.config.Architecture,
			OS:           image.config.OS,
			OSVersion:    image.config.OSVersion,
			OSFeatures:   image.config.OSFeatures,
		}),
	)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	// We check if the blob already exists.  It is possible to have a duplicate
	// configuration already present if Save() is called repeatedly.
	if exists, _ := image.handle.DigestExists(ctx, configBlob.desc.Digest); !exists {
		log.G(ctx).WithFields(logrus.Fields{
			"digest": configBlob.desc.Digest,
		}).Trace("oci: saving config")

		if _, err := image.AddBlob(ctx, configBlob); err != nil {
			return ocispec.Descriptor{}, err
		}
	}

	log.G(ctx).Trace("oci: packing image manifest")

	// Pack the given blobs which generates an image manifest for the pack, and
	// pushes it to a content storage.
	if image.annotations == nil {
		image.annotations = make(map[string]string)
	}

	// General annotations
	image.annotations[ocispec.AnnotationRefName] = ref.Context().String()
	image.annotations[ocispec.AnnotationRevision] = ref.Identifier()
	image.annotations[ocispec.AnnotationCreated] = time.Now().UTC().Format(time.RFC3339)

	// containerd compatibility annotations
	image.annotations[images.AnnotationImageName] = ref.String()

	// Generate the final manifest
	image.manifest = ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2, // historical value. does not pertain to OCI or docker version
		},
		Config:      configBlob.desc,
		MediaType:   ocispec.MediaTypeImageManifest,
		Layers:      layers,
		Annotations: image.annotations,
	}

	manifestJson, err := json.Marshal(image.manifest)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	image.manifestDesc = content.NewDescriptorFromBytes(
		ocispec.MediaTypeImageManifest,
		manifestJson,
	)
	image.manifestDesc.ArtifactType = image.manifest.Config.MediaType
	image.manifestDesc.Annotations = image.manifest.Annotations

	// save the manifest digest
	if err := image.handle.SaveDigest(
		ctx,
		source,
		image.manifestDesc,
		bytes.NewReader(manifestJson),
		onProgress,
	); err != nil && !errors.Is(err, errdefs.ErrAlreadyExists) {
		return ocispec.Descriptor{}, fmt.Errorf("failed to push manifest: %w", err)
	}

	return image.manifestDesc, nil
}

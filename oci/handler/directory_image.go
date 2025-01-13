// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"

	"kraftkit.sh/log"
)

const algorithm = "sha256"

type DirectoryLayer struct {
	ctx       context.Context
	path      string
	diffID    digest.Digest
	digest    v1.Hash
	size      int64
	mediatype types.MediaType
}

// Digest returns the digest of the layer as a Hash
func (dl DirectoryLayer) Digest() (v1.Hash, error) {
	return dl.digest, nil
}

// DiffID returns the diffID of the layer as a Hash
func (dl DirectoryLayer) DiffID() (v1.Hash, error) {
	return v1.NewHash(dl.diffID.String())
}

// Compressed returns the compressed layer as a ReadCloser
// It reads the layer from the filesystem
func (dl DirectoryLayer) Compressed() (io.ReadCloser, error) {
	layerPath := filepath.Join(
		dl.path,
		DirectoryHandlerDigestsDir,
		dl.digest.Algorithm,
		dl.digest.Hex,
	)

	reader, err := os.Open(layerPath)
	if err != nil {
		return nil, fmt.Errorf("opening layer: %w", err)
	}

	return reader, nil
}

// Uncompressed is not implemented
func (dl DirectoryLayer) Uncompressed() (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

// Size returns the size of the layer
func (dl DirectoryLayer) Size() (int64, error) {
	return dl.size, nil
}

// MediaType returns the mediatype of the layer
func (dl DirectoryLayer) MediaType() (types.MediaType, error) {
	layerPath := filepath.Join(
		dl.path,
		DirectoryHandlerDigestsDir,
		dl.digest.Algorithm,
		dl.digest.Hex,
	)

	if _, err := os.Open(layerPath); err != nil {
		log.G(dl.ctx).
			Debugf("error accessing layer: %s: marking as non-distributable layer", err.Error())
		return types.OCIRestrictedLayer, nil
	}

	return dl.mediatype, nil
}

type DirectoryManifest struct {
	ctx    context.Context
	handle *DirectoryHandler
	image  *ocispec.Image
	desc   *ocispec.Descriptor
}

// Layers returns the layers of the image
func (dm DirectoryManifest) Layers() ([]v1.Layer, error) {
	var layers []v1.Layer

	manifest, err := dm.Manifest()
	if err != nil {
		return nil, err
	}

	// Only works if the order is the same in the rootfs and the manifest
	for idx, layer := range manifest.Layers {
		dlayer := DirectoryLayer{
			ctx:       dm.ctx,
			path:      dm.handle.path,
			digest:    layer.Digest,
			diffID:    dm.image.RootFS.DiffIDs[idx],
			size:      layer.Size,
			mediatype: layer.MediaType,
		}

		layers = append(layers, dlayer)
	}

	return layers, nil
}

// MediaType returns the mediatype of the image
func (dm DirectoryManifest) MediaType() (types.MediaType, error) {
	return types.OCIManifestSchema1, nil
}

// Size returns the size of the image manifest
func (dm DirectoryManifest) Size() (int64, error) {
	return dm.desc.Size, nil
}

// ConfigName returns the hash of the image config
func (dm DirectoryManifest) ConfigName() (v1.Hash, error) {
	b, err := dm.RawConfigFile()
	if err != nil {
		return v1.Hash{}, err
	}
	h, _, err := v1.SHA256(bytes.NewReader(b))

	return h, err
}

// ConfigFile returns the structured config file of the image
func (dm DirectoryManifest) ConfigFile() (*v1.ConfigFile, error) {
	b, err := dm.RawConfigFile()
	if err != nil {
		return nil, err
	}

	return v1.ParseConfigFile(bytes.NewReader(b))
}

// RawConfigFile returns the config file of the image in bytes
// It reads the config file from the filesystem
func (dm DirectoryManifest) RawConfigFile() ([]byte, error) {
	bytes, err := json.Marshal(dm.image)
	if err != nil {
		return nil, err
	}

	h := sha256.New()
	_, err = h.Write(bytes)
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(
		dm.handle.path,
		DirectoryHandlerDigestsDir,
		algorithm,
		hex.EncodeToString(h.Sum(nil)),
	)

	// Check if the config file exists
	if _, err := os.Stat(configPath); err != nil {
		return nil, err
	}

	return os.ReadFile(configPath)
}

// Digest returns the hash of the image manifest
func (dm DirectoryManifest) Digest() (v1.Hash, error) {
	b, err := dm.RawManifest()
	if err != nil {
		return v1.Hash{}, err
	}

	h, _, err := v1.SHA256(bytes.NewReader(b))
	return h, err
}

// Manifest returns the structured manifest of the image
func (dm DirectoryManifest) Manifest() (*v1.Manifest, error) {
	b, err := dm.RawManifest()
	if err != nil {
		return nil, err
	}

	return v1.ParseManifest(bytes.NewReader(b))
}

// RawManifest returns the manifest of the image in bytes
// It reads the manifest from the filesystem
func (dm DirectoryManifest) RawManifest() ([]byte, error) {
	return os.ReadFile(filepath.Join(
		dm.handle.path,
		DirectoryHandlerDigestsDir,
		dm.desc.Digest.Algorithm().String(),
		dm.desc.Digest.Encoded(),
	))
}

// LayerByDigest returns the layer with the given hash
// Unused by push
func (dm DirectoryManifest) LayerByDigest(hash v1.Hash) (v1.Layer, error) {
	manifest, err := dm.Manifest()
	if err != nil {
		return nil, err
	}

	for idx, layer := range manifest.Layers {
		if layer.Digest == hash {
			// Only works if the order is the same in the rootfs and the manifest
			dlayer := DirectoryLayer{
				ctx:       dm.ctx,
				path:      dm.handle.path,
				diffID:    dm.image.RootFS.DiffIDs[idx],
				digest:    layer.Digest,
				size:      layer.Size,
				mediatype: layer.MediaType,
			}
			return dlayer, nil
		}
	}

	return nil, fmt.Errorf("layer not found")
}

// LayerByDiffID returns the layer with the given hash
// Unused by push
func (dm DirectoryManifest) LayerByDiffID(hash v1.Hash) (v1.Layer, error) {
	manifest, err := dm.Manifest()
	if err != nil {
		return nil, err
	}

	for idx, digest := range dm.image.RootFS.DiffIDs {
		hashStep, err := v1.NewHash(digest.String())
		if err != nil {
			return nil, err
		}

		if hashStep == hash {
			// Only works if the order is the same in the rootfs and the manifest
			dlayer := DirectoryLayer{
				path:      dm.handle.path,
				diffID:    dm.image.RootFS.DiffIDs[idx],
				digest:    manifest.Layers[idx].Digest,
				size:      manifest.Layers[idx].Size,
				mediatype: manifest.Layers[idx].MediaType,
			}
			return dlayer, nil
		}
	}
	return nil, fmt.Errorf("layer not found")
}

type DirectoryIndex struct {
	desc    *ocispec.Descriptor
	handle  *DirectoryHandler
	fullref string
	ctx     context.Context
}

// MediaType implements v1.ImageIndex
func (di *DirectoryIndex) MediaType() (types.MediaType, error) {
	return types.OCIImageIndex, nil
}

// Digest implements v1.ImageIndex
func (di *DirectoryIndex) Digest() (v1.Hash, error) {
	return v1.Hash{
		Algorithm: di.desc.Digest.Algorithm().String(),
		Hex:       di.desc.Digest.Hex(),
	}, nil
}

// Size implements v1.ImageIndex
func (di *DirectoryIndex) Size() (int64, error) {
	return di.desc.Size, nil
}

// IndexManifest implements v1.ImageIndex
func (di *DirectoryIndex) IndexManifest() (*v1.IndexManifest, error) {
	b, err := di.RawManifest()
	if err != nil {
		return nil, err
	}

	return v1.ParseIndexManifest(bytes.NewReader(b))
}

// RawManifest implements v1.ImageIndex
func (di *DirectoryIndex) RawManifest() ([]byte, error) {
	return os.ReadFile(filepath.Join(
		di.handle.path,
		DirectoryHandlerIndexesDir,
		strings.ReplaceAll(di.fullref, ":", string(filepath.Separator)),
	))
}

// Image implements v1.ImageIndex
func (di *DirectoryIndex) Image(manifestDigest v1.Hash) (v1.Image, error) {
	dgst := digest.NewDigestFromHex(manifestDigest.Algorithm, manifestDigest.Hex)

	manifest, image, err := di.handle.ResolveManifest(
		di.ctx,
		di.fullref,
		dgst,
	)
	if err != nil {
		return nil, fmt.Errorf("resolving manifest: %w", err)
	}

	indexJson, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	desc := content.NewDescriptorFromBytes(
		ocispec.MediaTypeImageIndex,
		indexJson,
	)

	return &DirectoryManifest{
		ctx:    di.ctx,
		handle: di.handle,
		image:  image,
		desc:   &desc,
	}, nil
}

// ImageIndex implements v1.ImageIndex
func (di *DirectoryIndex) ImageIndex(v1.Hash) (v1.ImageIndex, error) {
	return nil, fmt.Errorf("not implemented: oci.handler.DirectoryIndex.ImageIndex")
}

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

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type DirectoryLayer struct {
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
		DirectoryHandlerLayersDir,
		dl.digest.Algorithm,
		dl.digest.Hex,
	)

	reader, err := os.Open(layerPath)
	if err != nil {
		return nil, err
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
	return dl.mediatype, nil
}

type DirectoryImage struct {
	handle             *DirectoryHandler
	image              ocispec.Image
	manifestDescriptor *ocispec.Descriptor
	ref                name.Reference
}

// Layers returns the layers of the image
func (di DirectoryImage) Layers() ([]v1.Layer, error) {
	var layers []v1.Layer

	manifest, err := di.Manifest()
	if err != nil {
		return nil, err
	}

	// Only works if the order is the same in the rootfs and the manifest
	for idx, layer := range manifest.Layers {
		dlayer := DirectoryLayer{
			path:      di.handle.path,
			digest:    layer.Digest,
			diffID:    di.image.RootFS.DiffIDs[idx],
			size:      layer.Size,
			mediatype: layer.MediaType,
		}

		layers = append(layers, dlayer)
	}

	return layers, nil
}

// MediaType returns the mediatype of the image
func (di DirectoryImage) MediaType() (types.MediaType, error) {
	return types.MediaType(di.manifestDescriptor.MediaType), nil
}

// Size returns the size of the image manifest
func (di DirectoryImage) Size() (int64, error) {
	return di.manifestDescriptor.Size, nil
}

// ConfigName returns the hash of the image config
func (di DirectoryImage) ConfigName() (v1.Hash, error) {
	b, err := di.RawConfigFile()
	if err != nil {
		return v1.Hash{}, err
	}
	h, _, err := v1.SHA256(bytes.NewReader(b))

	return h, err
}

// ConfigFile returns the structured config file of the image
func (di DirectoryImage) ConfigFile() (*v1.ConfigFile, error) {
	b, err := di.RawConfigFile()
	if err != nil {
		return nil, err
	}

	return v1.ParseConfigFile(bytes.NewReader(b))
}

// RawConfigFile returns the config file of the image in bytes
// It reads the config file from the filesystem
func (di DirectoryImage) RawConfigFile() ([]byte, error) {
	bytes, err := json.Marshal(di.image)
	if err != nil {
		return nil, err
	}

	h := sha256.New()
	_, err = h.Write(bytes)
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(
		di.handle.path,
		DirectoryHandlerConfigsDir,
		string(di.manifestDescriptor.Digest.Algorithm()),
		hex.EncodeToString(h.Sum(nil)),
	)

	// Check if the config file exists
	if _, err := os.Stat(configPath); err != nil {
		return nil, err
	}

	return os.ReadFile(configPath)
}

// Digest returns the hash of the image manifest
func (di DirectoryImage) Digest() (v1.Hash, error) {
	b, err := di.RawManifest()
	if err != nil {
		return v1.Hash{}, err
	}

	h, _, err := v1.SHA256(bytes.NewReader(b))
	return h, err
}

// Manifest returns the structured manifest of the image
func (di DirectoryImage) Manifest() (*v1.Manifest, error) {
	b, err := di.RawManifest()
	if err != nil {
		return nil, err
	}

	return v1.ParseManifest(bytes.NewReader(b))
}

// RawManifest returns the manifest of the image in bytes
// It reads the manifest from the filesystem
func (di DirectoryImage) RawManifest() ([]byte, error) {
	manifestPath := filepath.Join(
		di.handle.path,
		DirectoryHandlerManifestsDir,
		string(di.manifestDescriptor.Digest.Algorithm()),
		di.manifestDescriptor.Digest.Encoded(),
	)

	return os.ReadFile(manifestPath)
}

// LayerByDigest returns the layer with the given hash
// Unused by push
func (di DirectoryImage) LayerByDigest(hash v1.Hash) (v1.Layer, error) {
	manifest, err := di.Manifest()
	if err != nil {
		return nil, err
	}

	for idx, layer := range manifest.Layers {
		if layer.Digest == hash {
			// Only works if the order is the same in the rootfs and the manifest
			dlayer := DirectoryLayer{
				path:      di.handle.path,
				diffID:    di.image.RootFS.DiffIDs[idx],
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
func (di DirectoryImage) LayerByDiffID(hash v1.Hash) (v1.Layer, error) {
	manifest, err := di.Manifest()
	if err != nil {
		return nil, err
	}

	for idx, digest := range di.image.RootFS.DiffIDs {
		hashStep, err := v1.NewHash(digest.String())
		if err != nil {
			return nil, err
		}

		if hashStep == hash {
			// Only works if the order is the same in the rootfs and the manifest
			dlayer := DirectoryLayer{
				path:      di.handle.path,
				diffID:    di.image.RootFS.DiffIDs[idx],
				digest:    manifest.Layers[idx].Digest,
				size:      manifest.Layers[idx].Size,
				mediatype: manifest.Layers[idx].MediaType,
			}
			return dlayer, nil
		}
	}
	return nil, fmt.Errorf("layer not found")
}

type DirectoryImageIndex struct {
	handle          *DirectoryHandler
	index           ocispec.Index
	indexDescriptor *ocispec.Descriptor
	ref             name.Reference
}

// MediaType of this image's manifest.
func (dix DirectoryImageIndex) MediaType() (types.MediaType, error) {
	return types.MediaType(dix.index.MediaType), nil
}

// Digest returns the sha256 of this index's manifest.
func (dix DirectoryImageIndex) Digest() (v1.Hash, error) {
	b, err := dix.RawManifest()
	if err != nil {
		return v1.Hash{}, err
	}

	h, _, err := v1.SHA256(bytes.NewReader(b))
	return h, err
}

// Size returns the size of the manifest.
func (dix DirectoryImageIndex) Size() (int64, error) {
	return dix.indexDescriptor.Size, nil
}

// IndexManifest returns this image index's manifest object.
func (dix DirectoryImageIndex) IndexManifest() (*v1.IndexManifest, error) {
	b, err := dix.RawManifest()
	if err != nil {
		return nil, err
	}

	return v1.ParseIndexManifest(bytes.NewReader(b))
}

// RawManifest returns the serialized bytes of IndexManifest().
func (dix DirectoryImageIndex) RawManifest() ([]byte, error) {
	var jsonPath string
	if strings.ContainsRune(dix.ref.Name(), '@') {
		jsonPath = strings.ReplaceAll(dix.ref.Name(), "@", string(filepath.Separator)) + ".json"
	} else {
		jsonPath = strings.ReplaceAll(dix.ref.Name(), ":", string(filepath.Separator)) + ".json"
	}

	indexPath := filepath.Join(
		dix.handle.path,
		DirectoryHandlerIndexesDir,
		jsonPath,
	)

	return os.ReadFile(indexPath)
}

// Image returns a v1.Image that this ImageIndex references.
func (dix DirectoryImageIndex) Image(hash v1.Hash) (v1.Image, error) {
	var manifest *ocispec.Descriptor

	for _, desc := range dix.index.Manifests {
		if desc.Digest.Encoded() == hash.Hex {
			desc := desc
			manifest = &desc
		}
	}

	// Fetch manifest and config from the filesystem
	image, err := dix.handle.ResolveImage(context.TODO(), manifest.Digest.Encoded(), "")
	if err != nil {
		return nil, err
	}

	return DirectoryImage{
		image:              image,
		manifestDescriptor: manifest,
		handle:             dix.handle,
		ref:                dix.ref,
	}, nil
}

// ImageIndex returns a v1.ImageIndex that this ImageIndex references.
func (dix DirectoryImageIndex) ImageIndex(hash v1.Hash) (v1.ImageIndex, error) {
	return nil, fmt.Errorf("mixed image indexes are not supported, contributions welcome")
}

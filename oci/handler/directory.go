// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package handler

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"kraftkit.sh/internal/version"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/simpleauth"

	regtypes "github.com/docker/docker/api/types/registry"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	DirectoryHandlerIndexesDir   = "indexes"
	DirectoryHandlerManifestsDir = "manifests"
	DirectoryHandlerConfigsDir   = "configs"
	DirectoryHandlerLayersDir    = "layers"
)

type DirectoryHandler struct {
	path  string
	auths map[string]regtypes.AuthConfig
}

func NewDirectoryHandler(path string, auths map[string]regtypes.AuthConfig) (*DirectoryHandler, error) {
	if err := os.MkdirAll(path, 0o775); err != nil {
		return nil, fmt.Errorf("could not create local oci cache directory: %w", err)
	}

	return &DirectoryHandler{
		path:  path,
		auths: auths,
	}, nil
}

// DigestExists implements DigestResolver.
func (handle *DirectoryHandler) DigestExists(ctx context.Context, dgst digest.Digest) (exists bool, err error) {
	manifests, err := handle.ListManifests(ctx)
	if err != nil {
		return false, err
	}

	for _, manifest := range manifests {
		if manifest.Config.Digest == dgst {
			return true, nil
		}
	}

	return false, nil
}

// ListIndexes implements DigestResolver.
func (handle *DirectoryHandler) ListIndexes(ctx context.Context) (indexes []ocispec.Index, err error) {
	indexesDir := filepath.Join(handle.path, DirectoryHandlerIndexesDir)

	// Create the manifest directory if it does not exist and return nil, since
	// there's nothing to return.
	if _, err := os.Stat(indexesDir); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(indexesDir, 0o775); err != nil {
			return nil, fmt.Errorf("could not create local oci cache directory: %w", err)
		}

		return nil, nil
	}

	// Since the directory structure is nested, recursively walk the manifest
	// directory to find all manifest entries.
	if err := filepath.WalkDir(indexesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip files that don't end in .json
		if !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}

		// Read the manifest
		rawIndex, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		index := ocispec.Index{}
		if err = json.Unmarshal(rawIndex, &index); err != nil {
			return err
		}

		// Append the manifest to the list
		indexes = append(indexes, index)

		return nil
	}); err != nil {
		return nil, err
	}

	return indexes, nil
}

// ListManifests implements DigestResolver.
func (handle *DirectoryHandler) ListManifests(ctx context.Context) (manifests []ocispec.Manifest, err error) {
	manifestsDir := filepath.Join(handle.path, DirectoryHandlerManifestsDir)

	// Create the manifest directory if it does not exist and return nil, since
	// there's nothing to return.
	if _, err := os.Stat(manifestsDir); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(manifestsDir, 0o775); err != nil {
			return nil, fmt.Errorf("could not create local oci cache directory: %w", err)
		}

		return nil, nil
	}

	// Since the directory structure is nested, recursively walk the manifest
	// directory to find all manifest entries.
	if err := filepath.WalkDir(manifestsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Read the manifest
		rawManifest, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		manifest := ocispec.Manifest{}
		if err = json.Unmarshal(rawManifest, &manifest); err != nil {
			return err
		}

		// Append the manifest to the list
		manifests = append(manifests, manifest)

		return nil
	}); err != nil {
		return nil, err
	}

	return manifests, nil
}

// progressWriter wraps an existing io.Reader and reports how much content has
// been written.
type progressWriter struct {
	io.Reader
	total      int
	onProgress func(float64)
}

// Read overrides the underlying io.Reader's Read method and injects the
// onProgress callback.
func (pt *progressWriter) Read(p []byte) (int, error) {
	n, err := pt.Reader.Read(p)
	pt.total += n
	pt.onProgress(float64(n / pt.total))
	return n, err
}

// SaveDigest implements DigestSaver.
func (handle *DirectoryHandler) SaveDigest(ctx context.Context, ref string, desc ocispec.Descriptor, reader io.Reader, onProgress func(float64)) error {
	blobPath := handle.path

	switch desc.MediaType {
	case ocispec.MediaTypeImageConfig:
		blobPath = filepath.Join(
			blobPath,
			DirectoryHandlerConfigsDir,
			desc.Digest.Algorithm().String(),
			desc.Digest.Encoded(),
		)
	case ocispec.MediaTypeImageManifest:
		blobPath = filepath.Join(
			blobPath,
			DirectoryHandlerManifestsDir,
			desc.Digest.Algorithm().String(),
			desc.Digest.Encoded(),
		)
	case ocispec.MediaTypeImageIndex:
		blobPath = filepath.Join(
			blobPath,
			DirectoryHandlerIndexesDir,
			strings.ReplaceAll(ref, ":", string(filepath.Separator))+".json",
		)
	case ocispec.MediaTypeImageLayer:
		fallthrough
	default:
		blobPath = filepath.Join(
			blobPath,
			DirectoryHandlerLayersDir,
			desc.Digest.Algorithm().String(),
			desc.Digest.Encoded(),
		)
	}

	// Create the parent directory if it does not exist
	if err := os.MkdirAll(filepath.Dir(blobPath), 0o774); err != nil {
		return fmt.Errorf("could not make parent directory: %w", err)
	}

	blob, err := os.OpenFile(blobPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o664)
	if err != nil {
		return fmt.Errorf("could not create blob: %w", err)
	}

	var progresReader io.Reader
	if onProgress != nil {
		progresReader = &progressWriter{
			Reader:     reader,
			onProgress: onProgress,
		}
	} else {
		progresReader = reader
	}

	if _, err := io.Copy(blob, progresReader); err != nil {
		return err
	}

	return nil
}

func (handle *DirectoryHandler) resolveIndex(_ context.Context, fullref string) (ocispec.Index, error) {
	// Check whether the reference is a digest
	if !strings.ContainsRune(fullref, '/') {
		return ocispec.Index{}, fmt.Errorf("invalid reference: %s", fullref)
	}

	ref, err := name.ParseReference(fullref)
	if err != nil {
		return ocispec.Index{}, err
	}

	var jsonPath string
	if strings.ContainsRune(ref.Name(), '@') {
		jsonPath = strings.ReplaceAll(ref.Name(), "@", string(filepath.Separator)) + ".json"
	} else {
		jsonPath = strings.ReplaceAll(ref.Name(), ":", string(filepath.Separator)) + ".json"
	}

	// Fetch the index
	indexPath := filepath.Join(
		handle.path,
		DirectoryHandlerIndexesDir,
		jsonPath,
	)

	// Check whether the index exists
	if _, err := os.Stat(indexPath); err != nil {
		return ocispec.Index{}, fmt.Errorf("index for %s does not exist: %s", ref.Name(), indexPath)
	}

	// Read the index
	reader, err := os.Open(indexPath)
	if err != nil {
		return ocispec.Index{}, err
	}
	defer reader.Close()

	indexRaw, err := io.ReadAll(reader)
	if err != nil {
		return ocispec.Index{}, err
	}

	// Unmarshal the index
	index := ocispec.Index{}
	if err = json.Unmarshal(indexRaw, &index); err != nil {
		return ocispec.Index{}, err
	}

	return index, nil
}

// ResolveImage fetches the image config from a given manifest reference.
// the reference is the manifest sha256 digest.
// ResolveImage implements ImageResolver.
func (handle *DirectoryHandler) ResolveImage(ctx context.Context, fullref, platform string) (imgspec ocispec.Image, err error) {
	var manifestPath string

	idx, err := handle.resolveIndex(ctx, fullref)
	if err == nil {
		var manifestDesc ocispec.Descriptor

		arch := strings.Split(platform, "/")[1]
		plat := strings.Split(platform, "/")[0]

		if arch == "" || plat == "" {
			return ocispec.Image{}, fmt.Errorf("incomplete platform: %s", platform)
		}

		for _, descriptor := range idx.Manifests {
			if descriptor.Platform.OS == plat && descriptor.Platform.Architecture == arch {
				manifestDesc = descriptor
				break
			}
		}

		if manifestDesc.Digest == "" {
			return ocispec.Image{}, fmt.Errorf("no manifest found for %s/%s", strings.Split(platform, "/")[0], strings.Split(platform, "/")[1])
		}

		// Split the digest into algorithm and hex
		manifestHash := v1.Hash{
			Algorithm: manifestDesc.Digest.Algorithm().String(),
			Hex:       manifestDesc.Digest.Encoded(),
		}

		manifestPath = filepath.Join(
			handle.path,
			DirectoryHandlerManifestsDir,
			manifestHash.Algorithm,
			manifestHash.Hex,
		)
	} else {
		// Check whether the reference is a digest
		if strings.ContainsRune(fullref, '/') {
			return ocispec.Image{}, fmt.Errorf("invalid reference: %s", fullref)
		}

		manifestPath = filepath.Join(
			handle.path,
			DirectoryHandlerManifestsDir,
			"sha256",
			fullref,
		)
	}

	// Check whether the manifest exists
	if _, err := os.Stat(manifestPath); err != nil {
		return ocispec.Image{}, fmt.Errorf("manifest for %s does not exist: %s", fullref, manifestPath)
	}

	// Read the manifest
	reader, err := os.Open(manifestPath)
	if err != nil {
		return ocispec.Image{}, err
	}
	defer reader.Close()

	manifestRaw, err := io.ReadAll(reader)
	if err != nil {
		return ocispec.Image{}, err
	}

	// Unmarshal the manifest
	manifest := ocispec.Manifest{}
	if err = json.Unmarshal(manifestRaw, &manifest); err != nil {
		return ocispec.Image{}, err
	}

	// Split the digest into algorithm and hex
	configHash := v1.Hash{
		Algorithm: manifest.Config.Digest.Algorithm().String(),
		Hex:       manifest.Config.Digest.Encoded(),
	}

	// Find the config file at the specified directory
	configDir := filepath.Join(
		handle.path,
		DirectoryHandlerConfigsDir,
		configHash.Algorithm,
		configHash.Hex,
	)

	// Check whether the config exists
	if _, err := os.Stat(configDir); err != nil {
		return ocispec.Image{}, fmt.Errorf("could not access config file for %s: %w", fullref, err)
	}

	// Read the config
	reader, err = os.Open(configDir)
	if err != nil {
		return ocispec.Image{}, err
	}

	configRaw, err := io.ReadAll(reader)
	if err != nil {
		return ocispec.Image{}, err
	}

	// Unmarshal the config
	config := ocispec.Image{}
	if err = json.Unmarshal(configRaw, &config); err != nil {
		return ocispec.Image{}, err
	}

	// Return the image
	return config, nil
}

// FetchImage implements ImageFetcher.
func (handle *DirectoryHandler) FetchImage(ctx context.Context, fullref, platform string, onProgress func(float64)) (err error) {
	plat := strings.Split(platform, "/")[0]
	arch := strings.Split(platform, "/")[1]

	ref, err := name.ParseReference(fullref)
	if err != nil {
		return err
	}

	authConfig := &authn.AuthConfig{}

	// Annoyingly convert between regtypes and authn.
	if auth, ok := handle.auths[ref.Context().RegistryStr()]; ok {
		authConfig.Auth = auth.Auth
		authConfig.IdentityToken = auth.IdentityToken
		authConfig.Password = auth.Password
		authConfig.RegistryToken = auth.RegistryToken
		authConfig.Username = auth.Username
	}

	idx, err := remote.Index(ref,
		remote.WithContext(ctx),
		remote.WithPlatform(v1.Platform{
			OS:           plat,
			Architecture: arch,
		}),
		remote.WithUserAgent(version.UserAgent()),
		remote.WithAuth(&simpleauth.SimpleAuthenticator{
			Auth: authConfig,
		}),
	)
	if err != nil {
		return err
	}

	// Write the index manifest
	manifest, err := idx.RawManifest()
	if err != nil {
		return err
	}

	digest, err := idx.Digest()
	if err != nil {
		return err
	}

	var jsonPath string
	if strings.ContainsRune(ref.Name(), '@') {
		jsonPath = strings.ReplaceAll(ref.Name(), "@", string(filepath.Separator)) + ".json"
	} else {
		jsonPath = strings.ReplaceAll(ref.Name(), ":", string(filepath.Separator)) + ".json"
	}

	manifestPath := filepath.Join(
		handle.path,
		DirectoryHandlerIndexesDir,
		jsonPath,
	)

	// Recursively create the directory
	if err = os.MkdirAll(manifestPath[:strings.LastIndex(manifestPath, "/")], 0o775); err != nil {
		return err
	}

	// Open a writer to the specified path
	writer, err := os.Create(manifestPath)
	if err != nil {
		return err
	}
	defer writer.Close()

	if _, err := writer.Write(manifest); err != nil {
		return err
	}

	parsableManifest, err := idx.IndexManifest()
	if err != nil {
		return err
	}

	var manifests []v1.Descriptor
	for _, descriptor := range parsableManifest.Manifests {
		if descriptor.Platform.OS == plat && descriptor.Platform.Architecture == arch {
			log.G(ctx).
				WithField("platform", fmt.Sprintf("%s/%s", descriptor.Platform.OS, descriptor.Platform.Architecture)).
				WithField("digest", descriptor.Digest).
				Trace("fetching")
			manifests = append(manifests, descriptor)
		} else {
			log.G(ctx).
				WithField("platform", fmt.Sprintf("%s/%s", descriptor.Platform.OS, descriptor.Platform.Architecture)).
				WithField("digest", descriptor.Digest).
				Trace("skip fetching")
		}
	}

	if len(manifests) == 0 {
		return fmt.Errorf("no manifest found for %s/%s", arch, plat)
	}

	for _, descriptor := range manifests {
		img, err := idx.Image(descriptor.Digest)
		if err != nil {
			return err
		}

		// Write the manifest
		manifest, err = img.RawManifest()
		if err != nil {
			return err
		}

		digest, err = img.Digest()
		if err != nil {
			return err
		}

		manifestPath = filepath.Join(
			handle.path,
			DirectoryHandlerManifestsDir,
			digest.Algorithm,
			digest.Hex,
		)

		// Recursively create the directory
		if err = os.MkdirAll(manifestPath[:strings.LastIndex(manifestPath, "/")], 0o775); err != nil {
			return err
		}

		// Open a writer to the specified path
		writer, err := os.Create(manifestPath)
		if err != nil {
			return err
		}
		defer writer.Close()

		if _, err := writer.Write(manifest); err != nil {
			return err
		}

		config, err := img.RawConfigFile()
		if err != nil {
			return err
		}

		configName, err := img.ConfigName()
		if err != nil {
			return err
		}

		configPath := filepath.Join(
			handle.path,
			DirectoryHandlerConfigsDir,
			configName.Algorithm,
			configName.Hex,
		)

		// If the config already exists, skip it
		if _, err := os.Stat(configPath); err == nil {
			return nil
		}

		// Recursively create the directory
		if err = os.MkdirAll(configPath[:strings.LastIndex(configPath, "/")], 0o775); err != nil {
			return err
		}

		writer, err = os.Create(configPath)
		if err != nil {
			return err
		}
		defer writer.Close()

		// Write the config
		if _, err = writer.Write(config); err != nil {
			return err
		}

		// Write the layers
		layers, err := img.Layers()
		if err != nil {
			return err
		}

		for _, layer := range layers {
			digest, err := layer.Digest()
			if err != nil {
				return err
			}

			layerPath := filepath.Join(
				handle.path,
				DirectoryHandlerLayersDir,
				digest.Algorithm,
				digest.Hex,
			)

			// Recursively create the directory
			if err = os.MkdirAll(layerPath[:strings.LastIndex(layerPath, "/")], 0o775); err != nil {
				return err
			}

			// If the layer already exists, skip it
			if _, err := os.Stat(layerPath); err == nil {
				continue
			}

			writer, err = os.Create(layerPath)
			if err != nil {
				return err
			}
			defer writer.Close()

			reader, err := layer.Compressed()
			if err != nil {
				return err
			}
			defer reader.Close()

			if _, err = io.Copy(writer, reader); err != nil {
				return err
			}
		}
	}

	return nil
}

// PushImage implements ImagePusher.
func (handle *DirectoryHandler) PushImage(ctx context.Context, fullref string, target *ocispec.Descriptor) error {
	ref, err := name.ParseReference(fullref)
	if err != nil {
		return err
	}

	authConfig := &authn.AuthConfig{}

	// Annoyingly convert between regtypes and authn.
	if auth, ok := handle.auths[ref.Context().RegistryStr()]; ok {
		authConfig.Auth = auth.Auth
		authConfig.IdentityToken = auth.IdentityToken
		authConfig.Password = auth.Password
		authConfig.RegistryToken = auth.RegistryToken
		authConfig.Username = auth.Username
	}

	index, err := handle.resolveIndex(ctx, fullref)
	if err != nil {
		return err
	}

	return remote.WriteIndex(ref,
		DirectoryImageIndex{
			index:           index,
			indexDescriptor: target,
			handle:          handle,
			ref:             ref,
		},
		remote.WithContext(ctx),
		remote.WithUserAgent(version.UserAgent()),
		remote.WithAuth(&simpleauth.SimpleAuthenticator{
			Auth: authConfig,
		}),
	)
}

// UnpackImage implements ImageUnpacker.
func (handle *DirectoryHandler) UnpackImage(ctx context.Context, ref string, platform string, dest string) (err error) {
	plat := strings.Split(platform, "/")[0]
	arch := strings.Split(platform, "/")[1]

	idx, err := handle.resolveIndex(ctx, ref)
	if err != nil {
		return err
	}

	var manifests []digest.Digest
	for _, descriptor := range idx.Manifests {
		if descriptor.Platform.Architecture == arch && descriptor.Platform.OS == plat {
			log.G(ctx).Tracef("Pick for unpacking %s/%s manifest %s",
				descriptor.Platform.Architecture,
				descriptor.Platform.OS,
				descriptor.Digest)
			manifests = append(manifests, descriptor.Digest)
		} else {
			log.G(ctx).Tracef("Skip unpacking %s/%s manifest %s",
				descriptor.Platform.Architecture,
				descriptor.Platform.OS,
				descriptor.Digest)
		}
	}

	if len(manifests) == 0 {
		return fmt.Errorf("no manifest found for platform %s", platform)
	} else if len(manifests) > 1 {
		return fmt.Errorf("multiple manifests found for platform %s, unpacking all would overwrite results", platform)
	}

	for _, manifest := range manifests {
		img, err := handle.ResolveImage(ctx, manifest.Encoded(), platform)
		if err != nil {
			return err
		}

		// Iterate over the layers
		for _, layer := range img.RootFS.DiffIDs {
			// Get the digest
			digest, err := v1.NewHash(layer.String())
			if err != nil {
				return err
			}

			// Get the layer path
			layerPath := filepath.Join(
				handle.path,
				DirectoryHandlerLayersDir,
				digest.Algorithm,
				digest.Hex,
			)

			// Layer path is a tarball, so we need to extract it
			reader, err := os.Open(layerPath)
			if err != nil {
				return err
			}

			defer reader.Close()

			tr := tar.NewReader(reader)

			for {
				hdr, err := tr.Next()
				if err != nil {
					break
				}

				// Write the file to the destination
				path := filepath.Join(dest, hdr.Name)

				// If the file is a directory, create it
				if hdr.Typeflag == tar.TypeDir {
					if err := os.MkdirAll(path, 0o775); err != nil {
						return err
					}
					continue
				}

				// If the directory in the path doesn't exist, create it
				if _, err := os.Stat(path[:strings.LastIndex(path, "/")]); os.IsNotExist(err) {
					if err := os.MkdirAll(path[:strings.LastIndex(path, "/")], 0o775); err != nil {
						return err
					}
				}

				// Otherwise, create the file
				writer, err := os.Create(path)
				if err != nil {
					return err
				}

				defer writer.Close()

				if _, err = io.Copy(writer, tr); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// FinalizeImage implements ImageFinalizer.
func (handle *DirectoryHandler) FinalizeImage(ctx context.Context, image ocispec.Image) error {
	return fmt.Errorf("not implemented: oci.handler.DirectoryHandler.FinalizeImage")
}

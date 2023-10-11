// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package handler

import (
	"archive/tar"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/version"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/simpleauth"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	DirectoryHandlerManifestsDir = "manifests"
	DirectoryHandlerConfigsDir   = "configs"
	DirectoryHandlerLayersDir    = "layers"
)

type DirectoryHandler struct {
	path  string
	auths map[string]config.AuthConfig
}

func NewDirectoryHandler(path string, auths map[string]config.AuthConfig) (*DirectoryHandler, error) {
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

// PullDigest implements DigestPuller.
func (handle *DirectoryHandler) PullDigest(ctx context.Context, mediaType, fullref string, dgst digest.Digest, plat *ocispec.Platform, onProgress func(float64)) error {
	ref, err := name.ParseReference(fullref)
	if err != nil {
		return err
	}

	authConfig := &authn.AuthConfig{}

	ropts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithUserAgent(version.UserAgent()),
		remote.WithPlatform(v1.Platform{
			Architecture: plat.Architecture,
			OS:           plat.OS,
			OSFeatures:   plat.OSFeatures,
		}),
	}

	// Annoyingly convert between regtypes and authn.
	if auth, ok := handle.auths[ref.Context().RegistryStr()]; ok {
		authConfig.Username = auth.User
		authConfig.Password = auth.Token

		ropts = append(ropts,
			remote.WithAuth(&simpleauth.SimpleAuthenticator{
				Auth: authConfig,
			}),
		)

		if !auth.VerifySSL {
			transport := remote.DefaultTransport.(*http.Transport).Clone()
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}

			ropts = append(ropts, remote.WithTransport(transport))
		}
	}

	switch mediaType {
	case ocispec.MediaTypeImageManifest:
		log.G(ctx).
			WithField("digest", dgst.String()).
			Debugf("pulling manifest")

		image, err := remote.Image(ref, ropts...)
		if err != nil {
			return fmt.Errorf("could not retrieve remote manifest: %w", err)
		}

		manifestRaw, err := image.RawManifest()
		if err != nil {
			return fmt.Errorf("could not get raw manifest: %w", err)
		}

		manifest := ocispec.Manifest{}
		if err = json.Unmarshal(manifestRaw, &manifest); err != nil {
			return fmt.Errorf("could not unmarshal raw manifest: %w", err)
		}

		manifestPath := filepath.Join(
			handle.path,
			DirectoryHandlerManifestsDir,
			dgst.Algorithm().String(),
			dgst.Encoded()+".json",
		)

		if err = os.MkdirAll(filepath.Dir(manifestPath), 0o775); err != nil {
			return fmt.Errorf("could not make manifest parent directories: %w", err)
		}

		manifestWriter, err := os.Create(manifestPath)
		if err != nil {
			return fmt.Errorf("could not get manifest file descriptor: %w", err)
		}

		defer manifestWriter.Close()

		if _, err = manifestWriter.Write(manifestRaw); err != nil {
			return fmt.Errorf("could not write manifest: %w", err)
		}

		configDgst, err := image.ConfigName()
		if err != nil {
			return fmt.Errorf("could not get config digest: %w", err)
		}

		log.G(ctx).
			WithField("digest", configDgst.String()).
			Debugf("pulling config")

		configRaw, err := image.RawConfigFile()
		if err != nil {
			return fmt.Errorf("could not get raw config: %w", err)
		}

		config := ocispec.Image{}
		if err = json.Unmarshal(configRaw, &config); err != nil {
			return fmt.Errorf("could not unmarshal raw config: %w", err)
		}

		configPath := filepath.Join(
			handle.path,
			DirectoryHandlerConfigsDir,
			configDgst.Algorithm,
			configDgst.Hex+".json",
		)

		if err = os.MkdirAll(filepath.Dir(configPath), 0o775); err != nil {
			return fmt.Errorf("could not create config parent directories: %w", err)
		}

		configWriter, err := os.Create(configPath)
		if err != nil {
			return fmt.Errorf("could not get config file descriptor: %w", err)
		}

		defer configWriter.Close()

		if _, err = configWriter.Write(configRaw); err != nil {
			return fmt.Errorf("could not write raw config: %w", err)
		}

		layers, err := image.Layers()
		if err != nil {
			return fmt.Errorf("could not get manifest layers: %w", err)
		}

		// First calculate the total size of all layers.  This is done so that the
		// onProgress callback correctly reports
		var totalSize int64
		for _, layer := range layers {
			size, err := layer.Size()
			if err != nil {
				return fmt.Errorf("could not get layer size: %w", err)
			}

			totalSize += size
		}

		for _, layer := range layers {
			layerDgst, err := layer.Digest()
			if err != nil {
				return fmt.Errorf("could not get layer digest: %w", err)
			}

			if err := handle.PullDigest(ctx,
				ocispec.MediaTypeImageLayer,
				fullref,
				digest.NewDigestFromHex(layerDgst.Algorithm, layerDgst.Hex),
				plat,
				func(size float64) {
					onProgress(size / float64(totalSize))
				},
			); err != nil {
				return fmt.Errorf("could not pull layer from digest: %w", err)
			}
		}

	case ocispec.MediaTypeImageLayer, ocispec.MediaTypeImageLayerGzip:
		log.G(ctx).
			WithField("digest", dgst.String()).
			Debugf("pulling layer")

		fullref = fmt.Sprintf("%s/%s@%s",
			ref.Context().RegistryStr(),
			ref.Context().RepositoryStr(),
			dgst.String(),
		)

		layerV1Dgst, err := name.NewDigest(fullref)
		if err != nil {
			return fmt.Errorf("could not generate full digest: %w", err)
		}

		layer, err := remote.Layer(layerV1Dgst, ropts...)
		if err != nil {
			return fmt.Errorf("could not get remote layer: %w", err)
		}

		mediaType, err := layer.MediaType()
		if err != nil {
			return fmt.Errorf("could not get media type of layer: %w", err)
		}

		var reader io.ReadCloser

		switch mediaType {
		case ocispec.MediaTypeImageLayer, types.DockerUncompressedLayer:
			reader, err = layer.Uncompressed()
		case ocispec.MediaTypeImageLayerGzip, types.DockerLayer:
			reader, err = layer.Compressed()
		default:
			return fmt.Errorf("unsupported layer mediatype '%s'", mediaType)
		}
		if err != nil {
			return fmt.Errorf("could not open layer reader: %w", err)
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

		layerPath := filepath.Join(
			handle.path,
			DirectoryHandlerLayersDir,
			dgst.Algorithm().String(),
			dgst.Encoded(),
		)

		if err = os.MkdirAll(filepath.Dir((layerPath)), 0o775); err != nil {
			return fmt.Errorf("could not make directory: %w", err)
		}

		blob, err := os.OpenFile(layerPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o664)
		if err != nil {
			return fmt.Errorf("could not create layer: %w", err)
		}

		if _, err := io.Copy(blob, progresReader); err != nil {
			return fmt.Errorf("could not write layer: %w", err)
		}

	default:
		return fmt.Errorf("cannot push descriptor: unsupported mediatype: %s", mediaType)
	}

	return nil
}

// ResolveManifest implements ManifestResolver.
func (handle *DirectoryHandler) ResolveManifest(ctx context.Context, fullref string, dgst digest.Digest) (*ocispec.Manifest, error) {
	manifestPath := filepath.Join(
		handle.path,
		DirectoryHandlerManifestsDir,
		dgst.Algorithm().String(),
		dgst.Encoded()+".json",
	)

	// Check whether the manifest exists
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, fmt.Errorf("manifest for '%s' does not exist: %s", dgst.String(), manifestPath)
	}

	// Read the manifest
	reader, err := os.Open(manifestPath)
	if err != nil {
		return nil, err
	}

	defer reader.Close()

	manifestRaw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Unmarshal the manifest
	manifest := ocispec.Manifest{}
	if err = json.Unmarshal(manifestRaw, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
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

		// Skip files that don't end in .json
		if !strings.HasSuffix(d.Name(), ".json") {
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

func (handle *DirectoryHandler) DeleteManifest(ctx context.Context, fullref string, dgst digest.Digest) error {
	manifestPath := filepath.Join(
		handle.path,
		DirectoryHandlerManifestsDir,
		dgst.Algorithm().String(),
		dgst.Hex()+".json",
	)

	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("manifest '%s' does not exist at '%s'", dgst.String(), manifestPath)
	}

	manifestReader, err := os.Open(manifestPath)
	if err != nil {
		return err
	}

	defer manifestReader.Close()

	manifestRaw, err := io.ReadAll(manifestReader)
	if err != nil {
		return err
	}

	manifest := ocispec.Manifest{}
	if err = json.Unmarshal(manifestRaw, &manifest); err != nil {
		return err
	}

	for _, layer := range manifest.Layers {
		layerPath := filepath.Join(
			handle.path,
			DirectoryHandlerLayersDir,
			layer.Digest.Algorithm().String(),
			layer.Digest.Hex(),
		)

		if err := os.RemoveAll(layerPath); err != nil {
			return fmt.Errorf("could not delete layer digest from manifest '%s': %w", dgst.String(), err)
		}
	}

	configPath := filepath.Join(
		handle.path,
		DirectoryHandlerConfigsDir,
		manifest.Config.Digest.Algorithm().String(),
		manifest.Config.Digest.Hex()+".json",
	)

	if err := os.RemoveAll(configPath); err != nil {
		return fmt.Errorf("could not delete config digest from manifest '%s': %w", dgst.String(), err)
	}

	// TODO(nderjung): Remove empty parent directories up until
	// DirectoryHandlerManifestsDir.
	return os.RemoveAll(manifestPath)
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

// ResolveImage implements ImageResolver.
func (handle *DirectoryHandler) ResolveImage(ctx context.Context, fullref string) (*ocispec.Image, error) {
	// Find the manifest of this image
	ref, err := name.ParseReference(fullref)
	if err != nil {
		return nil, err
	}

	var jsonPath string
	if strings.ContainsRune(ref.Name(), '@') {
		jsonPath = strings.ReplaceAll(ref.Name(), "@", string(filepath.Separator)) + ".json"
	} else {
		jsonPath = strings.ReplaceAll(ref.Name(), ":", string(filepath.Separator)) + ".json"
	}

	manifestPath := filepath.Join(
		handle.path,
		DirectoryHandlerManifestsDir,
		jsonPath,
	)

	// Check whether the manifest exists
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, fmt.Errorf("manifest for %s does not exist: %s", ref.Name(), manifestPath)
	}

	// Read the manifest
	reader, err := os.Open(manifestPath)
	if err != nil {
		return nil, err
	}

	manifestRaw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Unmarshal the manifest
	manifest := ocispec.Manifest{}
	if err = json.Unmarshal(manifestRaw, &manifest); err != nil {
		return nil, err
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
		return nil, fmt.Errorf("could not access config file for %s: %w", ref.Name(), err)
	}

	// Read the config
	reader, err = os.Open(configDir)
	if err != nil {
		return nil, err
	}

	configRaw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Unmarshal the config
	var image ocispec.Image
	if err = json.Unmarshal(configRaw, &image); err != nil {
		return nil, err
	}

	// Return the image
	return &image, nil
}

// FetchImage implements ImageFetcher.
func (handle *DirectoryHandler) FetchImage(ctx context.Context, fullref, platform string, onProgress func(float64)) (err error) {
	ref, err := name.ParseReference(fullref)
	if err != nil {
		return err
	}

	authConfig := &authn.AuthConfig{}

	// Annoyingly convert between regtypes and authn.
	if auth, ok := handle.auths[ref.Context().RegistryStr()]; ok {
		authConfig.Username = auth.User
		authConfig.Password = auth.Token
	}

	img, err := remote.Image(ref,
		remote.WithContext(ctx),
		remote.WithPlatform(v1.Platform{
			OS:           strings.Split(platform, "/")[0],
			Architecture: strings.Split(platform, "/")[1],
		}),
		remote.WithUserAgent(version.UserAgent()),
		remote.WithAuth(&simpleauth.SimpleAuthenticator{
			Auth: authConfig,
		}),
	)
	if err != nil {
		return err
	}

	// Write the manifest
	manifest, err := img.RawManifest()
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
		DirectoryHandlerManifestsDir,
		jsonPath,
	)

	// Recursively create the directory
	if err = os.MkdirAll(manifestPath[:strings.LastIndex(manifestPath, string(filepath.Separator))], 0o775); err != nil {
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
	if err = os.MkdirAll(configPath[:strings.LastIndex(configPath, string(filepath.Separator))], 0o775); err != nil {
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
		if err = os.MkdirAll(layerPath[:strings.LastIndex(layerPath, string(filepath.Separator))], 0o775); err != nil {
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

	return nil
}

// PushImage implements ImagePusher.
func (handle *DirectoryHandler) PushImage(ctx context.Context, fullref string, target *ocispec.Descriptor) error {
	ref, err := name.ParseReference(fullref)
	if err != nil {
		return err
	}

	image, err := handle.ResolveImage(ctx, fullref)
	if err != nil {
		return err
	}

	authConfig := &authn.AuthConfig{}

	// Annoyingly convert between regtypes and authn.
	if auth, ok := handle.auths[ref.Context().RegistryStr()]; ok {
		authConfig.Username = auth.User
		authConfig.Password = auth.Token
	}

	return remote.Write(ref,
		DirectoryManifest{
			image:  image,
			desc:   target,
			handle: handle,
		},
		remote.WithContext(ctx),
		remote.WithUserAgent(version.UserAgent()),
		remote.WithAuth(&simpleauth.SimpleAuthenticator{
			Auth: authConfig,
		}),
	)
}

// UnpackImage implements ImageUnpacker.
func (handle *DirectoryHandler) UnpackImage(ctx context.Context, ref string, dest string) (err error) {
	img, err := handle.ResolveImage(ctx, ref)
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
			if _, err := os.Stat(path[:strings.LastIndex(path, string(filepath.Separator))]); os.IsNotExist(err) {
				if err := os.MkdirAll(path[:strings.LastIndex(path, string(filepath.Separator))], 0o775); err != nil {
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

	return nil
}

// FinalizeImage implements ImageFinalizer.
func (handle *DirectoryHandler) FinalizeImage(ctx context.Context, image ocispec.Image) error {
	return fmt.Errorf("not implemented: oci.handler.DirectoryHandler.FinalizeImage")
}

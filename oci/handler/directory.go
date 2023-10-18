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
	"kraftkit.sh/internal/lockedfile"
	"kraftkit.sh/internal/set"
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
	DirectoryHandlerDigestsDir = "digests"
	DirectoryHandlerIndexesDir = "indexes"
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
func (handle *DirectoryHandler) DigestExists(ctx context.Context, needle digest.Digest) (exists bool, err error) {
	manifests, err := handle.ListManifests(ctx)
	if err != nil {
		return false, err
	}

	for haystack := range manifests {
		if haystack == needle.String() {
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
	case ocispec.MediaTypeImageIndex:
		log.G(ctx).
			WithField("digest", dgst.String()).
			Debugf("pulling index")

		indexV1, err := remote.Index(ref, ropts...)
		if err != nil {
			return fmt.Errorf("could not retrieve remote index: %w", err)
		}

		manifests := []ocispec.Descriptor{}
		var indexTagPath string

		if !strings.ContainsRune(fullref, '@') && len(strings.SplitN(fullref, ":", 2)) == 2 {
			indexTagPath = filepath.Join(
				handle.path,
				DirectoryHandlerIndexesDir,
				strings.ReplaceAll(fullref, ":", string(filepath.Separator)),
			)

			if indexFi, err := os.Stat(indexTagPath); err == nil {
				if indexFi.Mode()&fs.ModeSymlink == 0 {
					oldIndexDigestPath, err := filepath.EvalSymlinks(indexTagPath)
					if err != nil {
						return err
					}

					if err := os.RemoveAll(indexTagPath); err != nil {
						return fmt.Errorf("could not remove index symbolic link: %w", err)
					}

					// Check if a local index already exists, if it does we will append the
					// requested manifest to it.
					if _, err := os.Stat(oldIndexDigestPath); err == nil {
						localIndexRaw, err := lockedfile.Read(oldIndexDigestPath)
						if err != nil {
							return fmt.Errorf("could not read existing index: %w", err)
						}

						localIndex := ocispec.Index{}
						if err = json.Unmarshal(localIndexRaw, &localIndex); err != nil {
							return fmt.Errorf("could not unmarshal raw index: %w", err)
						}

						// Save the existing local manifests
						manifests = localIndex.Manifests

						if err := os.RemoveAll(oldIndexDigestPath); err != nil {
							return fmt.Errorf("could not remove existing index: %w", err)
						}
					}
				}
			}
		}

		var indexRaw []byte
		index := ocispec.Index{}

		indexRaw, err = indexV1.RawManifest()
		if err != nil {
			return fmt.Errorf("could not get raw index: %w", err)
		}

		if err = json.Unmarshal(indexRaw, &index); err != nil {
			return fmt.Errorf("could not unmarshal raw index: %w", err)
		}

		// Remove manifests that do not match the platform selector.
	checkManifest:
		for _, manifest := range index.Manifests {
			if plat.OS != "" && plat.OS != manifest.Platform.OS {
				continue
			}

			if plat.Architecture != "" && plat.Architecture != manifest.Platform.Architecture {
				continue
			}

			if len(plat.OSFeatures) > 0 {
				available := set.NewStringSet(manifest.Platform.OSFeatures...)

				// Iterate through the platform selector's requested set of features and
				// skip only if the descriptor does not contain the requested feature.
				for _, a := range plat.OSFeatures {
					if !available.Contains(a) {
						continue checkManifest
					}
				}
			}

			if err := handle.PullDigest(ctx,
				ocispec.MediaTypeImageManifest,
				fullref,
				manifest.Digest,
				plat,
				onProgress,
			); err != nil {
				return fmt.Errorf("could not pull manifest: %w", err)
			}

			manifests = append(manifests, manifest)
		}

		index.Manifests = manifests

		indexRaw, err = json.Marshal(&index)
		if err != nil {
			return fmt.Errorf("could not marshal raw index: %w", err)
		}

		newIndexDigest := digest.FromBytes(indexRaw)
		newIndexDigestPath := filepath.Join(
			handle.path,
			DirectoryHandlerDigestsDir,
			newIndexDigest.Algorithm().String(),
			newIndexDigest.Encoded(),
		)

		if err = os.MkdirAll(filepath.Dir(newIndexDigestPath), 0o775); err != nil {
			return fmt.Errorf("could not make manifest parent directories: %w", err)
		}

		indexWriter, err := lockedfile.Edit(newIndexDigestPath)
		if err != nil {
			return fmt.Errorf("could not get manifest file descriptor: %w", err)
		}

		defer indexWriter.Close()

		if _, err = indexWriter.Write(indexRaw); err != nil {
			return fmt.Errorf("could not write manifest: %w", err)
		}

		if len(indexTagPath) > 0 {
			// Create the parent directory if it does not exist
			if err := os.MkdirAll(filepath.Dir(indexTagPath), 0o774); err != nil {
				return fmt.Errorf("could not make parent directory: %w", err)
			}

			if err := os.Symlink(newIndexDigestPath, indexTagPath); err != nil {
				return err
			}
		}

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
			DirectoryHandlerDigestsDir,
			dgst.Algorithm().String(),
			dgst.Encoded(),
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
			DirectoryHandlerDigestsDir,
			configDgst.Algorithm,
			configDgst.Hex,
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
			DirectoryHandlerDigestsDir,
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

// SaveDescriptor implements DescriptorSaver.
func (handle *DirectoryHandler) SaveDescriptor(ctx context.Context, ref string, desc ocispec.Descriptor, reader io.Reader, onProgress func(float64)) error {
	blobPath := filepath.Join(
		handle.path,
		DirectoryHandlerDigestsDir,
		desc.Digest.Algorithm().String(),
		desc.Digest.Encoded(),
	)

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

	// Create a symbolic representing the tag if this is an index.
	switch desc.MediaType {
	case ocispec.MediaTypeImageIndex:
		if !strings.ContainsRune(ref, '@') && len(strings.SplitN(ref, ":", 2)) == 2 {
			indexTagPath := filepath.Join(
				handle.path,
				DirectoryHandlerIndexesDir,
				strings.ReplaceAll(ref, ":", string(filepath.Separator)),
			)

			if _, err := os.Stat(indexTagPath); err == nil {
				if err := os.RemoveAll(indexTagPath); err != nil {
					return fmt.Errorf("could not create symbolic link to new index: %w", err)
				}
			}

			// Create the parent directory if it does not exist
			if err := os.MkdirAll(filepath.Dir(indexTagPath), 0o774); err != nil {
				return fmt.Errorf("could not make parent directory: %w", err)
			}

			if err := os.Symlink(blobPath, indexTagPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// PushDescriptor implements DescriptorPusher.
func (handle *DirectoryHandler) PushDescriptor(ctx context.Context, fullref string, desc *ocispec.Descriptor) error {
	ref, err := name.ParseReference(fullref)
	if err != nil {
		return err
	}

	ropts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithUserAgent(version.UserAgent()),
	}

	authConfig := &authn.AuthConfig{}

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

	switch desc.MediaType {
	case ocispec.MediaTypeImageIndex:
		log.G(ctx).
			WithField("digest", desc.Digest.String()).
			Debugf("pushing index")

		return remote.WriteIndex(ref,
			&DirectoryIndex{
				ctx:     ctx,
				desc:    desc,
				handle:  handle,
				fullref: fullref,
			},
			ropts...,
		)

	case ocispec.MediaTypeImageManifest:
		log.G(ctx).
			WithField("digest", desc.Digest.String()).
			Debugf("pushing manifest")
		image, err := handle.resolveImage(ctx, fullref, desc.Digest)
		if err != nil {
			return err
		}

		return remote.Write(ref,
			DirectoryManifest{
				image:  image,
				desc:   desc,
				handle: handle,
			},
			append(ropts, remote.WithPlatform(v1.Platform{
				Architecture: image.Architecture,
				OS:           image.OS,
				OSVersion:    image.OSVersion,
				OSFeatures:   image.OSFeatures,
				Variant:      image.Variant,
			}))...,
		)

	// NOTE(nderjung): The manifest writer is able to handle pushing layers.
	// Currently within KraftKit, pushing a layer directly is not used and
	// therefore this code is not implemented.
	// case ocispec.MediaTypeImageLayer:
	// 	log.G(ctx).Debugf("pushing layer-%s", desc.Digest.String())

	default:
		return fmt.Errorf("cannot push descriptor: unsupported mediatype: %s", desc.MediaType)
	}
}

// ResolveManifest implements ManifestResolver.
func (handle *DirectoryHandler) ResolveManifest(ctx context.Context, fullref string, dgst digest.Digest) (*ocispec.Manifest, error) {
	manifestPath := filepath.Join(
		handle.path,
		DirectoryHandlerDigestsDir,
		dgst.Algorithm().String(),
		dgst.Encoded(),
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
func (handle *DirectoryHandler) ListManifests(ctx context.Context) (map[string]*ocispec.Manifest, error) {
	manifestsDir := filepath.Join(handle.path, DirectoryHandlerDigestsDir)
	manifests := map[string]*ocispec.Manifest{}

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
			return nil
		}

		manifest := ocispec.Manifest{}
		if err = json.Unmarshal(rawManifest, &manifest); err != nil {
			return nil
		}

		// Append the manifest to the list
		manifests[digest.FromBytes(rawManifest).String()] = &manifest

		return nil
	}); err != nil {
		return nil, fmt.Errorf("could not walk manifests directory: %w", err)
	}

	return manifests, nil
}

func (handle *DirectoryHandler) DeleteManifest(ctx context.Context, fullref string, dgst digest.Digest) error {
	manifestPath := filepath.Join(
		handle.path,
		DirectoryHandlerDigestsDir,
		dgst.Algorithm().String(),
		dgst.Hex(),
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
			DirectoryHandlerDigestsDir,
			layer.Digest.Algorithm().String(),
			layer.Digest.Hex(),
		)

		if err := os.RemoveAll(layerPath); err != nil {
			return fmt.Errorf("could not delete layer digest from manifest '%s': %w", dgst.String(), err)
		}
	}

	configPath := filepath.Join(
		handle.path,
		DirectoryHandlerDigestsDir,
		manifest.Config.Digest.Algorithm().String(),
		manifest.Config.Digest.Hex(),
	)

	if err := os.RemoveAll(configPath); err != nil {
		return fmt.Errorf("could not delete config digest from manifest '%s': %w", dgst.String(), err)
	}

	// Update the index manifest such that the specific manifests do not exit.  If
	// there are no more manifests in the index, also remove the index.
	index, err := handle.ResolveIndex(ctx, fullref)
	if err != nil {
		return fmt.Errorf("could not resolve index from manifest: %w", err)
	}

	var manifests []ocispec.Descriptor

	for _, m := range index.Manifests {
		if m.Digest.String() == dgst.String() {
			continue
		}

		manifests = append(manifests, m)
	}

	indexPath := filepath.Join(
		handle.path,
		DirectoryHandlerIndexesDir,
		strings.ReplaceAll(fullref, ":", string(filepath.Separator)),
	)

	if len(manifests) == 0 {
		indexFi, err := os.Stat(indexPath)
		if err != nil {
			return fmt.Errorf("could not stat index: %w", err)
		}

		if indexFi.Mode()&fs.ModeSymlink == 0 {
			indexDigest, err := filepath.EvalSymlinks(indexPath)
			if err != nil {
				return err
			}

			if err := os.RemoveAll(indexDigest); err != nil {
				return err
			}
		}

		// TODO(nderjung): Remove empty parent directories up until
		// DirectoryHandlerIndexesDir.

		if err := os.RemoveAll(indexPath); err != nil {
			return fmt.Errorf("could not delete index '%s': %w", fullref, err)
		}
	} else {
		index.Manifests = manifests

		indexJson, err := json.Marshal(index)
		if err != nil {
			return fmt.Errorf("could not marshal new index: %w", err)
		}

		indexFile, err := os.OpenFile(indexPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o664)
		if err != nil {
			return fmt.Errorf("could not open index file: %w", err)
		}

		defer indexFile.Close()

		if _, err := indexFile.Write(indexJson); err != nil {
			return fmt.Errorf("could not write index file: %w", err)
		}
	}

	// TODO(nderjung): Remove empty parent directories up until
	// DirectoryHandlerManifestsDir.
	return os.RemoveAll(manifestPath)
}

// ResolveIndex implements IndexResolver.
func (handle *DirectoryHandler) ResolveIndex(ctx context.Context, fullref string) (*ocispec.Index, error) {
	// Find the index of this image
	ref, err := name.ParseReference(fullref,
		name.WithDefaultRegistry(""),
	)
	if err != nil {
		return nil, err
	}

	var indexPath string
	if strings.Contains(fullref, "@") {
		indexPath = filepath.Join(
			handle.path,
			DirectoryHandlerIndexesDir,
			// TODO: Do not hardcode
			strings.ReplaceAll(ref.Name(), "@"+digest.SHA256.String()+":", string(filepath.Separator)),
		)
	} else {
		indexPath = filepath.Join(
			handle.path,
			DirectoryHandlerIndexesDir,
			strings.ReplaceAll(ref.Name(), ":", string(filepath.Separator)),
		)
	}

	// Check whether the index exists
	if _, err := os.Stat(indexPath); err != nil {
		return nil, fmt.Errorf("index '%s' not found", ref.Name())
	}

	// Read the index
	reader, err := os.Open(indexPath)
	if err != nil {
		return nil, err
	}

	defer reader.Close()

	indexRaw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Unmarshal the index
	index := ocispec.Index{}
	if err = json.Unmarshal(indexRaw, &index); err != nil {
		return nil, err
	}

	return &index, nil
}

// ListIndexes implements IndexLister.
func (handle *DirectoryHandler) ListIndexes(ctx context.Context) (map[string]*ocispec.Index, error) {
	indexesDir := filepath.Join(handle.path, DirectoryHandlerIndexesDir)
	indexes := map[string]*ocispec.Index{}

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

		info, err := d.Info()
		if err != nil {
			return nil
		}

		imageName := strings.TrimPrefix(path, filepath.Join(handle.path, DirectoryHandlerIndexesDir)+"/")

		if info.Mode()&fs.ModeSymlink == 0 {
			path, err = filepath.EvalSymlinks(path)
			if err != nil {
				return nil
			}
		}

		// Read the manifest
		rawIndex, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		index := ocispec.Index{}
		if err = json.Unmarshal(rawIndex, &index); err != nil {
			return nil
		}

		split := strings.Split(imageName, "/")
		identifier := split[len(split)-1]
		imageName = fmt.Sprintf("%s:%s", strings.Join(split[:len(split)-1], "/"), identifier)
		indexes[imageName] = &index

		return nil
	}); err != nil {
		return nil, err
	}

	return indexes, nil
}

func (handle *DirectoryHandler) DeleteIndex(ctx context.Context, fullref string) error {
	indexPath := filepath.Join(
		handle.path,
		DirectoryHandlerIndexesDir,
		strings.ReplaceAll(fullref, ":", string(filepath.Separator)),
	)

	// Check whether the index exists
	indexFi, err := os.Stat(indexPath)
	if err != nil {
		return nil
	}

	// Read the index
	reader, err := os.Open(indexPath)
	if err != nil {
		return err
	}

	defer reader.Close()

	indexRaw, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	index := ocispec.Index{}
	if err = json.Unmarshal(indexRaw, &index); err != nil {
		return err
	}

	for _, manifest := range index.Manifests {
		if err := handle.DeleteManifest(ctx, fullref, manifest.Digest); err != nil {
			return fmt.Errorf("could not delete manifest from '%s': %w", fullref, err)
		}
	}

	// TODO(nderjung): Remove empty parent directories up until
	// DirectoryHandlerIndexesDir.

	if _, err := os.Stat(indexPath); err == nil {
		if indexFi.Mode()&fs.ModeSymlink == 0 {
			indexDigest, err := filepath.EvalSymlinks(indexPath)
			if err != nil {
				return err
			}

			if err := os.RemoveAll(indexDigest); err != nil {
				return err
			}
		}

		return os.RemoveAll(indexPath)
	}

	return nil
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

func (handle *DirectoryHandler) resolveImage(ctx context.Context, fullref string, dgst digest.Digest) (*ocispec.Image, error) {
	// Find the manifest of this image
	ref, err := name.ParseReference(fullref)
	if err != nil {
		return nil, err
	}

	manifest, err := handle.ResolveManifest(ctx, fullref, dgst)
	if err != nil {
		return nil, fmt.Errorf("could not resolve image via manifest: %w", err)
	}

	// Split the digest into algorithm and hex
	configHash := v1.Hash{
		Algorithm: manifest.Config.Digest.Algorithm().String(),
		Hex:       manifest.Config.Digest.Encoded(),
	}

	// Find the config file at the specified directory
	configDir := filepath.Join(
		handle.path,
		DirectoryHandlerDigestsDir,
		configHash.Algorithm,
		configHash.Hex,
	)

	// Check whether the config exists
	if _, err := os.Stat(configDir); err != nil {
		return nil, fmt.Errorf("could not access config file for %s: %w", ref.Name(), err)
	}

	// Read the config
	reader, err := os.Open(configDir)
	if err != nil {
		return nil, err
	}

	defer reader.Close()

	configRaw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	// Unmarshal the config
	config := ocispec.Image{}
	if err = json.Unmarshal(configRaw, &config); err != nil {
		return nil, err
	}

	// Return the image
	return &config, nil
}

// UnpackImage implements ImageUnpacker.
func (handle *DirectoryHandler) UnpackImage(ctx context.Context, fullref string, dgst digest.Digest, dest string) (*ocispec.Image, error) {
	img, err := handle.resolveImage(ctx, fullref, dgst)
	if err != nil {
		return nil, err
	}

	// Iterate over the layers
	for _, layer := range img.RootFS.DiffIDs {
		// Get the digest
		digest, err := v1.NewHash(layer.String())
		if err != nil {
			return nil, err
		}

		// Get the layer path
		layerPath := filepath.Join(
			handle.path,
			DirectoryHandlerDigestsDir,
			digest.Algorithm,
			digest.Hex,
		)

		// Layer path is a tarball, so we need to extract it
		reader, err := os.Open(layerPath)
		if err != nil {
			return nil, err
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
					return nil, err
				}
				continue
			}

			// If the directory in the path doesn't exist, create it
			if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
				if err := os.MkdirAll(filepath.Dir(path), 0o775); err != nil {
					return nil, err
				}
			}

			// Otherwise, create the file
			writer, err := os.Create(path)
			if err != nil {
				return nil, err
			}

			defer writer.Close()

			if _, err = io.Copy(writer, tr); err != nil {
				return nil, err
			}
		}
	}

	return img, nil
}

// FinalizeImage implements ImageFinalizer.
func (handle *DirectoryHandler) FinalizeImage(ctx context.Context, image ocispec.Image) error {
	return fmt.Errorf("not implemented: oci.handler.DirectoryHandler.FinalizeImage")
}

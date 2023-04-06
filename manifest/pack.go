// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"fmt"

	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
)

type mpack struct {
	manifest *Manifest
	version  string
}

const ManifestFormat pack.PackageFormat = "manifest"

// NewPackageFromManifestWithVersion generates a new package based on an input
// manifest which in itself may contain various versions and channels.  With the
// provided version as a positional parameter, the manifest can be reduced to
// represent a specific version.
func NewPackageFromManifestWithVersion(ctx context.Context, manifest *Manifest, version string, opts ...ManifestOption) (pack.Package, error) {
	// Apply the options to this manifest
	for _, opt := range opts {
		if err := opt(manifest); err != nil {
			return nil, err
		}
	}

	var channels []ManifestChannel
	var versions []ManifestVersion

	// Tear down the manifest such that it only represents specific version
	for _, channel := range manifest.Channels {
		if channel.Name == version {
			channels = append(channels, channel)
		}
	}

	for _, ver := range manifest.Versions {
		if ver.Version == version {
			// resource = ver.Resource
			versions = append(versions, ver)
		}
	}

	manifest.Channels = channels
	manifest.Versions = versions

	if len(channels) == 0 && len(versions) == 0 {
		return nil, fmt.Errorf("unknown version: %s", version)
	}

	return &mpack{manifest, version}, nil
}

// NewPackageFromManifest generates a manifest implementation of the
// pack.Package construct based on the input Manifest using its default channel
func NewPackageFromManifest(ctx context.Context, manifest *Manifest, opts ...ManifestOption) (pack.Package, error) {
	channel, err := manifest.DefaultChannel()
	if err != nil {
		return nil, err
	}

	return NewPackageFromManifestWithVersion(ctx, manifest, channel.Name, opts...)
}

func (mp mpack) Type() unikraft.ComponentType {
	return mp.manifest.Type
}

func (mp mpack) Name() string {
	return mp.manifest.Name
}

func (mp mpack) Version() string {
	return mp.version
}

func (mp mpack) Metadata() any {
	return mp.manifest
}

func (mp mpack) Push(ctx context.Context, opts ...pack.PushOption) error {
	return fmt.Errorf("not implemented: manifest.ManifestPackage.Push")
}

func (mp mpack) Pull(ctx context.Context, opts ...pack.PullOption) error {
	log.G(ctx).Debugf("pulling manifest package %s", mp.Name())

	if mp.manifest.Provider == nil {
		return fmt.Errorf("uninitialized manifest provider")
	}

	opts = append(opts, pack.WithPullVersion(mp.version))

	return mp.manifest.Provider.PullManifest(ctx, mp.manifest, opts...)
}

// resourceCacheChecksum returns the resource path, checksum and the cache
// location for a given Manifestt which only has one channel or one version.  If
// the Manifest has more than one, then it is not possible to determine which
// resource should be looked up.
func resourceCacheChecksum(manifest *Manifest) (string, string, string, error) {
	var err error
	var resource string
	var checksum string
	var cache string

	if len(manifest.Channels) == 1 {
		resource = manifest.Channels[0].Resource
		checksum = manifest.Channels[0].Sha256
		cache = manifest.Channels[0].Local
	} else if len(manifest.Versions) == 1 {
		resource = manifest.Versions[0].Resource
		checksum = manifest.Versions[0].Sha256
		cache = manifest.Versions[0].Local
	} else {
		err = fmt.Errorf("too many options")
	}

	return resource, cache, checksum, err
}

func (mp mpack) Format() pack.PackageFormat {
	return ManifestFormat
}

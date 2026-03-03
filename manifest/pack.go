// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"kraftkit.sh/internal/tableprinter"
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
func NewPackageFromManifestWithVersion(manifest *Manifest, version string, opts ...ManifestOption) (pack.Package, error) {
	var channels []ManifestChannel
	var versions []ManifestVersion

	// Tear down the manifest such that it only represents specific version
	for _, channel := range manifest.Channels {
		if channel.Name == version {
			channels = append(channels, channel)
		}
	}

	for _, ver := range manifest.Versions {
		if ver.Version == version || ver.Unikraft == version {
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
func NewPackageFromManifest(manifest *Manifest, opts ...ManifestOption) (pack.Package, error) {
	channel, err := manifest.DefaultChannel()
	if err != nil {
		return nil, err
	}

	return NewPackageFromManifestWithVersion(manifest, channel.Name, opts...)
}

func (mp mpack) Type() unikraft.ComponentType {
	return mp.manifest.Type
}

func (mp mpack) Name() string {
	return mp.manifest.Name
}

func (mp mpack) ID() string {
	return fmt.Sprintf("%s/%s:%s", mp.manifest.Type, mp.manifest.Name, mp.version)
}

// Name implements fmt.Stringer
func (mp mpack) String() string {
	return mp.manifest.Name
}

func (mp mpack) Version() string {
	return mp.version
}

func (mp mpack) Metadata() interface{} {
	return mp.manifest
}

func (mp mpack) Size() int64 {
	return -1 // not implemented
}

func (mp mpack) Columns() []tableprinter.Column {
	return []tableprinter.Column{
		{Name: "description", Value: mp.manifest.Description},
		{Name: "origin", Value: mp.manifest.Origin},
	}
}

func (mp mpack) Push(ctx context.Context, opts ...pack.PushOption) error {
	return fmt.Errorf("not implemented: manifest.ManifestPackage.Push")
}

func (mp mpack) Unpack(ctx context.Context, dir string) error {
	return fmt.Errorf("not implemented: manifest.ManifestPackage.Unpack")
}

func (mp mpack) Pull(ctx context.Context, opts ...pack.PullOption) error {
	log.G(ctx).
		WithField("package", unikraft.TypeNameVersion(mp)).
		Debugf("pulling manifest")

	if mp.manifest.Provider == nil {
		return fmt.Errorf("uninitialized manifest provider")
	}

	if len(mp.manifest.Channels) == 1 {
		return mp.manifest.Provider.PullChannel(ctx, mp.manifest, &mp.manifest.Channels[0], opts...)
	} else if len(mp.manifest.Versions) == 1 {
		return mp.manifest.Provider.PullVersion(ctx, mp.manifest, &mp.manifest.Versions[0], opts...)
	}

	return fmt.Errorf("cannot determine which channel or version to pull")
}

func (mp mpack) PulledAt(context.Context) (bool, time.Time, error) {
	manifests, err := mp.manifest.Provider.Manifests()
	if err != nil {
		return false, time.Time{}, err
	}

	pulled := false
	earliest := time.Now()

	for _, manifest := range manifests {
		for _, channel := range manifest.Channels {
			cache := manifest.Name + string(filepath.Separator) + filepath.Base(channel.Resource)

			if manifest.Type != unikraft.ComponentTypeCore {
				cache = manifest.Type.Plural() + string(filepath.Separator) + cache
			}

			cache = filepath.Join(manifest.mopts.cacheDir, cache)

			si, err := os.Stat(cache)
			if err != nil {
				continue
			}

			pulled = true

			if earliest.Before(si.ModTime()) {
				earliest = si.ModTime()
			}
		}

		for _, version := range manifest.Versions {
			cache := manifest.Name + string(filepath.Separator) + filepath.Base(version.Resource)

			if manifest.Type != unikraft.ComponentTypeCore {
				cache = manifest.Type.Plural() + string(filepath.Separator) + cache
			}

			cache = filepath.Join(manifest.mopts.cacheDir, cache)

			si, err := os.Stat(cache)
			if err != nil {
				continue
			}

			pulled = true

			if earliest.Before(si.ModTime()) {
				earliest = si.ModTime()
			}
		}
	}

	if pulled {
		return true, earliest, nil
	}

	return false, time.Time{}, nil
}

func (mp mpack) CreatedAt(context.Context) (time.Time, error) {
	// TODO(nderjung): Need to determine the creation time of the manifest by
	// supplementing the manifest with a creation time field.
	return time.Time{}, nil
}

func (mp mpack) UpdatedAt(context.Context) (time.Time, error) {
	// TODO(nderjung): Need to determine the update time of the manifest by
	// supplementing the manifest with an update time field.
	return time.Time{}, nil
}

// Delete implements pack.Package.
func (mp mpack) Delete(ctx context.Context) error {
	return mp.manifest.Provider.DeleteManifest(ctx)
}

// Save implements pack.Package.
func (mp mpack) Save(ctx context.Context) error {
	return nil
}

// Format implements pack.Package
func (mp mpack) Export(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}

func (mp mpack) Format() pack.PackageFormat {
	return ManifestFormat
}

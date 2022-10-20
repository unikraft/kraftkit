// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package manifest

import (
	"context"
	"fmt"

	"kraftkit.sh/pack"
)

type ManifestPackage struct {
	*pack.PackageOptions
	Provider

	ctx context.Context
}

const (
	ManifestContext pack.ContextKey = "manifest"
)

// NewPackageFromOptions generates a manifest implementation of the pack.Package
// construct based on the input options
func NewPackageFromOptions(ctx context.Context, opts *pack.PackageOptions, provider Provider) (pack.Package, error) {
	if ctx == nil {
		return nil, fmt.Errorf("cannot create NewPackageFromOptions without context")
	}

	return ManifestPackage{
		PackageOptions: opts,
		Provider:       provider,
		ctx:            ctx,
	}, nil
}

// NewPackageWithVersion generates a manifest implementation of the pack.Package
// construct based on the input Manifest for a particular version
func NewPackageWithVersion(ctx context.Context, manifest *Manifest, version string, popts ...pack.PackageOption) (pack.Package, error) {
	resource := ""

	var channels []ManifestChannel
	var versions []ManifestVersion

	// Tear down the manifest such that it only represents specific version
	for _, channel := range manifest.Channels {
		if channel.Name == version {
			channels = append(channels, channel)
			resource = channel.Resource
		}
	}

	for _, ver := range manifest.Versions {
		if ver.Version == version {
			resource = ver.Resource
			versions = append(versions, ver)
		}
	}

	manifest.Channels = channels
	manifest.Versions = versions

	// Save the full manifest within the context via the `ContextKey`
	ctx = context.WithValue(
		ctx,
		ManifestContext,
		manifest,
	)

	popts = append(popts,
		pack.WithName(manifest.Name),
		pack.WithRemoteLocation(resource),
		pack.WithType(manifest.Type),
		pack.WithVersion(version),
	)

	pkgOpts, err := pack.NewPackageOptions(popts...)
	if err != nil {
		return nil, fmt.Errorf("could not prepare package for target: %s", err)
	}

	return NewPackageFromOptions(ctx, pkgOpts, manifest.Provider)
}

// NewPackageFromManifest generates a manifest implementation of the
// pack.Package construct based on the input Manifest
func NewPackageFromManifest(ctx context.Context, manifest *Manifest, popts ...pack.PackageOption) (pack.Package, error) {
	channel, err := manifest.DefaultChannel()
	if err != nil {
		return nil, err
	}

	return NewPackageWithVersion(ctx, manifest, channel.Name, popts...)
}

func (mp ManifestPackage) ApplyOptions(opts ...pack.PackageOption) error {
	for _, o := range opts {
		if err := o(mp.PackageOptions); err != nil {
			return err
		}
	}

	return nil
}

func (mp ManifestPackage) Options() *pack.PackageOptions {
	return mp.PackageOptions
}

func (mp ManifestPackage) Name() string {
	return mp.PackageOptions.Name
}

func (mp ManifestPackage) CanonicalName() string {
	return "manifest://" + string(mp.PackageOptions.Type) + "/" + mp.PackageOptions.Name + ":" + mp.PackageOptions.Version
}

func (mp ManifestPackage) Pack() error {
	return fmt.Errorf("not implemented: pack.manifest.Pack")
}

func (mp ManifestPackage) Compatible(ref string) bool {
	return false
}

func (mp ManifestPackage) Pull(opts ...pack.PullPackageOption) error {
	mp.Log().Infof("pulling manifest package %s", mp.CanonicalName())

	popts, err := pack.NewPullPackageOptions(opts...)
	if err != nil {
		return err
	}

	manifest := mp.ctx.Value(ManifestContext).(*Manifest)
	if manifest == nil {
		return fmt.Errorf("package does not contain manifest context")
	}

	return mp.PullPackage(manifest, mp.PackageOptions, popts)
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

func (mp ManifestPackage) String() string {
	return "manifest"
}

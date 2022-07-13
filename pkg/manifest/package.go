// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft UG.  All rights reserved.
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

	"go.unikraft.io/kit/pkg/pkg"
)

type ManifestPackage struct {
	*pkg.PackageOptions
}

const (
	ManifestContext pkg.ContextKey = "manifest"
)

// NewPackageFromOptions generates a manifest implementation of the pkg.Package
// construct based on the input options
func NewPackageFromOptions(opts *pkg.PackageOptions) (pkg.Package, error) {
	return ManifestPackage{opts}, nil
}

// NewPackageWithVersion generates a manifest implementation of the pkg.Package
// construct based on the input Manifest for a particular version
func NewPackageWithVersion(manifest *Manifest, version string) (pkg.Package, error) {
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
	ctx := context.WithValue(
		context.TODO(),
		ManifestContext,
		manifest,
	)

	pkgOpts, err := pkg.NewPackageOptions(
		[]pkg.PackageOption{
			pkg.WithContext(ctx),
			pkg.WithName(manifest.Name),
			pkg.WithRemoteLocation(resource),
			pkg.WithType(manifest.Type),
			pkg.WithVersion(version),
		}...,
	)
	if err != nil {
		return nil, fmt.Errorf("could not prepare package for target: %s", err)
	}

	return NewPackageFromOptions(pkgOpts)
}

// NewPackageFromManifest generates a manifest implementation of the pkg.Package
// construct based on the input Manifest
func NewPackageFromManifest(manifest *Manifest) (pkg.Package, error) {
	channel, err := manifest.DefaultChannel()
	if err != nil {
		return nil, err
	}

	return NewPackageWithVersion(manifest, channel.Name)
}

func (mp ManifestPackage) ApplyOptions(opts ...pkg.PackageOption) error {
	for _, o := range opts {
		if err := o(mp.PackageOptions); err != nil {
			return err
		}
	}

	return nil
}

func (mp ManifestPackage) Options() *pkg.PackageOptions {
	return mp.PackageOptions
}

func (mp ManifestPackage) Compatible(ref string) bool {
	return false
}

func (mp ManifestPackage) String() string {
	return "manifest"
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package manifest

import (
	"fmt"

	"kraftkit.sh/pack"
)

type Provider interface {
	// Manifests returns a slice of Manifests which can be returned by this
	// Provider
	Manifests() ([]*Manifest, error)

	// Pull from the provider
	PullPackage(*Manifest, *pack.PackageOptions, *pack.PullPackageOptions) error

	// String returns the name of the provider
	fmt.Stringer
}

var providerRequestCache []byte

// NewProvider ultimately returns one of the supported manifest providers by
// attempting an ordered instantiation based on the input source.  For the
// provider which does not return an error is indicator that it is supported and
// thus the return of NewProvider a compatible interface Provider able to gather
// information about the manifest.
func NewProvider(path string, mopts ...ManifestOption) (Provider, error) {
	providerRequestCache = nil
	provider, err := NewManifestIndexProvider(path, mopts...)
	if err == nil {
		return provider, nil
	}

	provider, err = NewManifestProvider(path, mopts...)
	if err == nil {
		return provider, nil
	}

	provider, err = NewGitHubProvider(path, mopts...)
	if err == nil {
		return provider, nil
	}

	provider, err = NewGitProvider(path, mopts...)
	if err == nil {
		return provider, nil
	}

	provider, err = NewDirectoryProvider(path, mopts...)
	if err == nil {
		return provider, nil
	}

	return nil, fmt.Errorf("could not determine provider for: %s", path)
}

// NewProvidersFromString returns a provider based on a giving string which
// identifies the provider
func NewProvidersFromString(provider, path string, mopts ...ManifestOption) (Provider, error) {
	switch provider {
	case "index":
		return NewManifestIndexProvider(path, mopts...)
	case "manifest":
		return NewManifestProvider(path, mopts...)
	case "github":
		return NewGitHubProvider(path, mopts...)
	case "git":
		return NewGitProvider(path, mopts...)
	case "directory":
		return NewDirectoryProvider(path, mopts...)
	}

	return nil, fmt.Errorf("could not determine provider for: %s", path)
}

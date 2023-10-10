// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package manifest

import (
	"kraftkit.sh/config"
)

// ManifestOptions contains a set of additional configuration necessary for the
// underlying ManifestProvider implementation that are not attributes of the
// provider itself.
type ManifestOptions struct {
	// cacheDir sets the location of component archives which are stored locally.
	cacheDir string

	// auths is an internal property set by a ManifestOption which is used by the
	// Manifest to access information a bout itself aswell as downloading a given
	// resource
	auths map[string]config.AuthConfig

	// update is a switch to enable or prevent remote connections when
	// instantiating a Manifest(Index) or when probing sources, versions, etc.
	update bool

	// opts saves the options that were used to instantiated this ManifestOptions
	// struct.
	opts []ManifestOption
}

type ManifestOption func(*ManifestOptions)

// NewManifestOptions returns an instantiated ManifestOptions based on variable
// input number of ManifestOption methods.
func NewManifestOptions(opts ...ManifestOption) *ManifestOptions {
	mopts := ManifestOptions{
		opts: opts, // Save the list of options in case they are required elsewhere.
	}
	for _, opt := range opts {
		opt(&mopts)
	}
	return &mopts
}

// WithAuthConfig sets any required authentication necessary for accessing the
// manifest.
func WithAuthConfig(auths map[string]config.AuthConfig) ManifestOption {
	return func(mopts *ManifestOptions) {
		mopts.auths = auths
	}
}

// WithCacheDir is an option which helps find cached Manifest Channel or Version
// resources.  When set to a directory, the fixed structure of this directory
// should allow us to look up (and also store) resources here for later use.
func WithCacheDir(dir string) ManifestOption {
	return func(mopts *ManifestOptions) {
		mopts.cacheDir = dir
	}
}

// WithUpdate is an option to indicate that remote network connections
// can be made when instantiating a Manifest(Index) or to allow probing remote
// sources to determine versions, etc.
func WithUpdate(update bool) ManifestOption {
	return func(mopts *ManifestOptions) {
		mopts.update = update
	}
}

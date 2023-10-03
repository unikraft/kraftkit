// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/log"
	"kraftkit.sh/pack"
)

type Provider interface {
	// Manifests returns a slice of Manifests which can be returned by this
	// Provider
	Manifests() ([]*Manifest, error)

	// PullManifest from the provider.
	PullManifest(context.Context, *Manifest, ...pack.PullOption) error

	// String returns the name of the provider
	fmt.Stringer
}

// NewProvider ultimately returns one of the supported manifest providers by
// attempting an ordered instantiation based on the input source.  For the
// provider which does not return an error is indicator that it is supported and
// thus the return of NewProvider a compatible interface Provider able to gather
// information about the manifest.
func NewProvider(ctx context.Context, path string, mopts ...ManifestOption) (Provider, error) {
	log.G(ctx).WithFields(logrus.Fields{
		"path": path,
	}).Trace("trying manifest provider")
	provider, err := NewManifestProvider(ctx, path, mopts...)
	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"path": path,
		}).Trace("using manifest provider")
		return provider, nil
	}

	log.G(ctx).WithFields(logrus.Fields{
		"path": path,
	}).Trace("trying index provider")
	provider, err = NewManifestIndexProvider(ctx, path, mopts...)
	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"path": path,
		}).Trace("using index provider")
		return provider, nil
	}

	log.G(ctx).WithFields(logrus.Fields{
		"path": path,
	}).Trace("trying directory provider")
	provider, err = NewDirectoryProvider(ctx, path, mopts...)
	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"path": path,
		}).Trace("using directory provider")
		return provider, nil
	}

	log.G(ctx).WithFields(logrus.Fields{
		"path": path,
	}).Trace("trying github provider")
	provider, err = NewGitHubProvider(ctx, path, mopts...)
	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"path": path,
		}).Trace("using github provider")
		return provider, nil
	}

	log.G(ctx).WithFields(logrus.Fields{
		"path": path,
	}).Trace("trying git provider")
	provider, err = NewGitProvider(ctx, path, mopts...)
	if err == nil {
		log.G(ctx).WithFields(logrus.Fields{
			"path": path,
		}).Trace("using git provider")
		return provider, nil
	}

	return nil, fmt.Errorf("could not determine provider for: %s", path)
}

// NewProviderFromString returns a provider based on a giving string which
// identifies the provider
func NewProviderFromString(ctx context.Context, provider, path string, entity any, mopts ...ManifestOption) (Provider, error) {
	switch provider {
	case "index":
		return &ManifestIndexProvider{
			path:  path,
			index: entity.(*ManifestIndex),
			mopts: mopts,
			ctx:   ctx,
		}, nil
	case "manifest":
		return &ManifestProvider{
			path:     path,
			manifest: entity.(*Manifest),
		}, nil
	case "github":
		return NewGitHubProvider(ctx, path, mopts...)
	case "git":
		return NewGitProvider(ctx, path, mopts...)
	case "directory", "dir":
		return NewDirectoryProvider(ctx, path, mopts...)
	}

	return nil, fmt.Errorf("could not determine provider for: %s", path)
}

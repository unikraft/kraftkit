// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package handler

import (
	"context"
	"io"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type DigestResolver interface {
	DigestExists(context.Context, digest.Digest) (bool, error)
}

type DigestPuller interface {
	// PullDigest retrieves the provided mediaType, full canonically referencable
	// image and its digest for the given platform and returns the progress of
	// retrieving said digest via the onProgress callback.
	PullDigest(ctx context.Context, mediaType, fullref string, dgst digest.Digest, plat *ocispec.Platform, onProgress func(float64)) error
}

type DigestSaver interface {
	SaveDigest(context.Context, string, ocispec.Descriptor, io.Reader, func(float64)) error
}

type DescriptorResolver interface {
	ResolveDescriptor(context.Context, string) (ocispec.Descriptor, error)
}

type ManifestLister interface {
	ListManifests(context.Context) ([]ocispec.Manifest, error)
}

type ManifestResolver interface {
	ResolveManifest(context.Context, string, digest.Digest) (*ocispec.Manifest, error)
}

type ManifestDeleter interface {
	DeleteManifest(context.Context, string, digest.Digest) error
}

type ImagePusher interface {
	PushImage(context.Context, string, *ocispec.Descriptor) error
}

type ImageResolver interface {
	ResolveImage(context.Context, string) (*ocispec.Image, error)
}

type ImageFetcher interface {
	FetchImage(context.Context, string, string, func(float64)) error
}

type ImageUnpacker interface {
	UnpackImage(context.Context, string, string) error
}

type Handler interface {
	DigestResolver
	DigestPuller
	DigestSaver
	ManifestLister
	ManifestResolver
	ManifestDeleter
	ImagePusher
	ImageResolver
	ImageFetcher
	ImageUnpacker
}

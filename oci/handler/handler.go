// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package handler

import (
	"context"
	"io"

	"github.com/containerd/containerd/content"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type DigestResolver interface {
	DigestInfo(context.Context, digest.Digest) (*content.Info, error)
}

type DigestPuller interface {
	// PullDigest retrieves the provided mediaType, full canonically referencable
	// image and its digest for the given platform and returns the progress of
	// retrieving said digest via the onProgress callback.
	PullDigest(ctx context.Context, mediaType, fullref string, dgst digest.Digest, plat *ocispec.Platform, onProgress func(float64)) error
}

type DescriptorSaver interface {
	// SaveDescriptor accepts an optional name reference which represents
	// descriptor (but this is not always necessary and can be left blank if the
	// descriptor is unnamed, e.g. an untagged config, a layer, etc) as well as an
	// io.Reader which is prepared to pass in the byte slice of the descriptor.
	// An optional progress method callback can be provided which is used to
	// deliver the progress of writing the descriptor by the implementing method.
	SaveDescriptor(context.Context, string, ocispec.Descriptor, io.Reader, func(float64)) error
}

type DescriptorPusher interface {
	// PushDescriptor accepts an input descriptor and an optional canonical name
	// for the descriptor (such as a tag) and uses the handler to push this to a
	// remote registry.
	PushDescriptor(context.Context, string, *ocispec.Descriptor) error
}

type ManifestLister interface {
	ListManifests(context.Context) (map[string]*ocispec.Manifest, error)
}

type ManifestResolver interface {
	ResolveManifest(context.Context, string, digest.Digest) (*ocispec.Manifest, *ocispec.Image, error)
}

type ManifestDeleter interface {
	DeleteManifest(context.Context, string, digest.Digest) error
}

type IndexLister interface {
	ListIndexes(context.Context) (map[string]*ocispec.Index, error)
}

type IndexResolver interface {
	ResolveIndex(context.Context, string) (*ocispec.Index, error)
}

type IndexDeleter interface {
	DeleteIndex(context.Context, string, bool) error
}

type ImageUnpacker interface {
	UnpackImage(context.Context, string, digest.Digest, string) (*ocispec.Image, error)
}

type Handler interface {
	DigestResolver
	DigestPuller
	DescriptorSaver
	DescriptorPusher
	ManifestLister
	ManifestResolver
	ManifestDeleter
	IndexResolver
	IndexLister
	IndexDeleter
	ImageUnpacker
}

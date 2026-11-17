// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package oci

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// failingHandler errors all local lookups, forcing resolution through the
// remote registry.
type failingHandler struct{}

func (failingHandler) DigestInfo(context.Context, digest.Digest) (*content.Info, error) {
	return nil, fmt.Errorf("not implemented")
}

func (failingHandler) PullDigest(context.Context, string, string, digest.Digest, *ocispec.Platform, func(float64)) error {
	return fmt.Errorf("not implemented")
}

func (failingHandler) ReadDigest(context.Context, digest.Digest) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not implemented")
}

func (failingHandler) SaveDescriptor(context.Context, string, ocispec.Descriptor, io.Reader, func(float64)) error {
	return fmt.Errorf("not implemented")
}

func (failingHandler) PushDescriptor(context.Context, string, *ocispec.Descriptor, func(float64)) error {
	return fmt.Errorf("not implemented")
}

func (failingHandler) ListManifests(context.Context) (map[string]*ocispec.Manifest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (failingHandler) ListDigests(context.Context) ([]digest.Digest, error) {
	return nil, fmt.Errorf("not implemented")
}

func (failingHandler) DeleteDigest(context.Context, digest.Digest) error {
	return fmt.Errorf("not implemented")
}

func (failingHandler) ResolveManifest(context.Context, string, digest.Digest) (*ocispec.Manifest, *ocispec.Image, error) {
	return nil, nil, fmt.Errorf("not implemented")
}

func (failingHandler) DeleteManifest(context.Context, string, digest.Digest) error {
	return fmt.Errorf("not implemented")
}

func (failingHandler) ListIndexes(context.Context) (map[string]*ocispec.Index, error) {
	return nil, fmt.Errorf("not implemented")
}

func (failingHandler) ResolveIndex(context.Context, string) (*ocispec.Index, digest.Digest, error) {
	return nil, "", fmt.Errorf("not implemented")
}

func (failingHandler) DeleteIndex(context.Context, string, bool) error {
	return fmt.Errorf("not implemented")
}

func (failingHandler) UnpackImage(context.Context, string, digest.Digest, string) (*ocispec.Image, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestNewIndexAndManifestFromRemoteDigest(t *testing.T) {
	ctx := context.Background()

	reg := httptest.NewServer(registry.New())
	defer reg.Close()

	repo := strings.TrimPrefix(reg.URL, "http://") + "/test/base"
	ref, err := name.ParseReference(repo + ":latest")
	if err != nil {
		t.Fatalf("could not parse reference: %v", err)
	}

	platforms := []v1.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
		{OS: "darwin", Architecture: "arm64"},
		{OS: "linux", Architecture: "riscv64"},
		{OS: "linux", Architecture: "s390x"},
	}

	index := mutate.IndexMediaType(empty.Index, types.OCIImageIndex)
	want := make([]string, 0, len(platforms))
	for i := range platforms {
		img, err := random.Image(32, 1)
		if err != nil {
			t.Fatalf("could not create random image: %v", err)
		}

		h, err := img.Digest()
		if err != nil {
			t.Fatalf("could not get image digest: %v", err)
		}

		want = append(want, h.String())
		index = mutate.AppendManifests(index, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				Platform: &platforms[i],
			},
		})
	}

	if err := remote.WriteIndex(ref, index); err != nil {
		t.Fatalf("could not write index: %v", err)
	}

	// Iterate to increase the chance that concurrent accesses overlap;
	// the race detector is what flags unsynchronized access.
	for i := 0; i < 10; i++ {
		gotIndex, gotManifest, err := newIndexAndManifestFromRemoteDigest(ctx, failingHandler{}, repo+":latest", nil, digest.Digest(want[1]))
		if err != nil {
			t.Fatalf("could not resolve index and manifest: %v", err)
		}

		if len(gotIndex.manifests) != len(want) {
			t.Fatalf("expected %d manifests, got %d", len(want), len(gotIndex.manifests))
		}

		for j, manifest := range gotIndex.manifests {
			if got := manifest.desc.Digest.String(); got != want[j] {
				t.Fatalf("manifest %d: expected digest %s, got %s", j, want[j], got)
			}

			if manifest.desc.Platform == nil ||
				manifest.desc.Platform.OS != platforms[j].OS ||
				manifest.desc.Platform.Architecture != platforms[j].Architecture {
				t.Fatalf("manifest %d: expected platform %s/%s, got %+v", j, platforms[j].OS, platforms[j].Architecture, manifest.desc.Platform)
			}
		}

		if gotManifest == nil {
			t.Fatalf("expected manifest for digest %s, got nil", want[1])
		}

		if got := gotManifest.desc.Digest.String(); got != want[1] {
			t.Fatalf("expected manifest digest %s, got %s", want[1], got)
		}

		if gotManifest != gotIndex.manifests[1] {
			t.Fatalf("expected selected manifest to be index entry 1")
		}
	}
}

// mixedHandler resolves two digests locally and forces the rest through
// the remote registry, exercising both collection branches in one call.
type mixedHandler struct {
	failingHandler
	local map[digest.Digest]bool
}

func (h mixedHandler) ResolveManifest(_ context.Context, _ string, dgst digest.Digest) (*ocispec.Manifest, *ocispec.Image, error) {
	if !h.local[dgst] {
		return nil, nil, fmt.Errorf("not available locally")
	}

	return &ocispec.Manifest{}, &ocispec.Image{}, nil
}

func TestNewIndexAndManifestFromRemoteDigestMixedLocalRemote(t *testing.T) {
	ctx := context.Background()

	reg := httptest.NewServer(registry.New())
	defer reg.Close()

	repo := strings.TrimPrefix(reg.URL, "http://") + "/test/base"
	ref, err := name.ParseReference(repo + ":latest")
	if err != nil {
		t.Fatalf("could not parse reference: %v", err)
	}

	platforms := []v1.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
		{OS: "darwin", Architecture: "arm64"},
	}

	index := mutate.IndexMediaType(empty.Index, types.OCIImageIndex)
	want := make([]string, 0, len(platforms))
	for i := range platforms {
		img, err := random.Image(32, 1)
		if err != nil {
			t.Fatalf("could not create random image: %v", err)
		}

		h, err := img.Digest()
		if err != nil {
			t.Fatalf("could not get image digest: %v", err)
		}

		want = append(want, h.String())
		index = mutate.AppendManifests(index, mutate.IndexAddendum{
			Add: img,
			Descriptor: v1.Descriptor{
				Platform: &platforms[i],
			},
		})
	}

	if err := remote.WriteIndex(ref, index); err != nil {
		t.Fatalf("could not write index: %v", err)
	}

	// Entries 0 and 2 resolve locally; entry 1 (the selected one) is
	// fetched from the registry.
	handle := mixedHandler{local: map[digest.Digest]bool{
		digest.Digest(want[0]): true,
		digest.Digest(want[2]): true,
	}}

	for i := 0; i < 10; i++ {
		gotIndex, gotManifest, err := newIndexAndManifestFromRemoteDigest(ctx, handle, repo+":latest", nil, digest.Digest(want[1]))
		if err != nil {
			t.Fatalf("could not resolve index and manifest: %v", err)
		}

		if len(gotIndex.manifests) != len(want) {
			t.Fatalf("expected %d manifests, got %d", len(want), len(gotIndex.manifests))
		}

		for j, manifest := range gotIndex.manifests {
			if got := manifest.desc.Digest.String(); got != want[j] {
				t.Fatalf("manifest %d: expected digest %s, got %s", j, want[j], got)
			}

			if handle.local[digest.Digest(want[j])] {
				// Locally resolved manifests carry no layers and their
				// config platform is overwritten from the descriptor.
				if len(manifest.layers) != 0 {
					t.Fatalf("manifest %d: expected no layers on the local path, got %d", j, len(manifest.layers))
				}

				if manifest.config == nil ||
					manifest.config.Platform.OS != platforms[j].OS ||
					manifest.config.Platform.Architecture != platforms[j].Architecture {
					t.Fatalf("manifest %d: expected config platform %s/%s, got %+v", j, platforms[j].OS, platforms[j].Architecture, manifest.config)
				}
			} else if len(manifest.layers) == 0 {
				t.Fatalf("manifest %d: expected layers on the remote path, got none", j)
			}
		}

		if gotManifest == nil || gotManifest != gotIndex.manifests[1] {
			t.Fatalf("expected selected manifest to be index entry 1")
		}
	}
}

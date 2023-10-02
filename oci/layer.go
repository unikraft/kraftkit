// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import (
	"context"
	"os"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"kraftkit.sh/archive"
)

type Layer struct {
	dst  string
	tmp  string
	blob *Blob
}

// NewLayerFromFile creates a new layer from a given blob
func NewLayerFromFile(ctx context.Context, mediaType, src, dst string, opts ...LayerOption) (*Layer, error) {
	if mediaType == "" {
		mediaType = ocispec.MediaTypeImageLayer
	}

	layer := Layer{dst: dst}

	removeAfterSave := false

	switch mediaType {
	case ocispec.MediaTypeImageLayer,
		MediaTypeImageKernelGzip,
		MediaTypeImageKernel:

		tmp, err := os.CreateTemp("", "kraftkit-ociblob*")
		if err != nil {
			return nil, err
		}

		if err := archive.TarFileTo(ctx,
			src, dst, tmp.Name(),
			archive.WithStripTimes(true),
			archive.WithGzip(mediaType == MediaTypeImageKernelGzip),
		); err != nil {
			return nil, err
		}

		src = tmp.Name()

		if err := tmp.Close(); err != nil {
			if os.Remove(tmp.Name()); err != nil {
				return nil, err
			}

			return nil, err
		}

		removeAfterSave = true
		layer.tmp = src
	}

	blob, err := NewBlobFromFile(ctx, mediaType, src,
		WithBlobRemoveAfterSave(removeAfterSave),
	)
	if err != nil {
		return nil, err
	}

	layer.blob = blob

	for _, opt := range opts {
		if err := opt(&layer); err != nil {
			return nil, err
		}
	}

	return &layer, nil
}

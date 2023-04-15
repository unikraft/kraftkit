// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import (
	"context"
	"fmt"
	"os"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Blob struct {
	desc            ocispec.Descriptor
	src             string
	tmp             string // intermediate location of the blob
	removeAfterSave bool
}

// NewBlob generates an OCI blob based on input byte array for a given media
// type.
func NewBlob(_ context.Context, mediaType string, data []byte, opts ...BlobOption) (*Blob, error) {
	if mediaType == "" {
		return nil, fmt.Errorf("unknown blob type")
	}

	fi, err := os.CreateTemp("", "kraftkit_oci-*")
	if err != nil {
		return nil, err
	}

	defer fi.Close()

	if _, err := fi.Write(data); err != nil {
		return nil, err
	}

	blob := Blob{
		src: fi.Name(),
		tmp: fi.Name(),
		desc: ocispec.Descriptor{
			MediaType: mediaType,
			Digest:    digest.FromBytes(data),
			Size:      int64(len(data)),
		},
	}

	for _, opt := range opts {
		if err := opt(&blob); err != nil {
			return nil, err
		}
	}

	return &blob, nil
}

// NewBlobFromFile generates an OCI blob based on an input file for a given
// media type
func NewBlobFromFile(_ context.Context, mediaType string, filePath string, opts ...BlobOption) (*Blob, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat %s: %v", filePath, err)
	}

	fp, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", filePath, err)
	}

	defer func() {
		closeErr := fp.Close()
		if err == nil {
			err = closeErr
		}
	}()

	dgst, err := digest.FromReader(fp)
	if err != nil {
		return nil, err
	}

	if mediaType == "" {
		mediaType = ocispec.MediaTypeImageLayer
	}

	blob := Blob{
		src: filePath,
		tmp: filePath,
		desc: ocispec.Descriptor{
			MediaType: mediaType,
			Digest:    dgst,
			Size:      fi.Size(),
		},
	}

	for _, opt := range opts {
		if err := opt(&blob); err != nil {
			return nil, err
		}
	}

	return &blob, nil
}

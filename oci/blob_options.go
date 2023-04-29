// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import ocispec "github.com/opencontainers/image-spec/specs-go/v1"

type BlobOption func(*Blob) error

// WithBlobRemoveAfterSave atomizes each operation on the blob.
func WithBlobRemoveAfterSave(removeAfterSave bool) BlobOption {
	return func(blob *Blob) error {
		blob.removeAfterSave = removeAfterSave
		return nil
	}
}

// WithBlobPlatform specifies platform attribution such that the later queries
// to the blob store which include platform specification only return those with
// the set parameters.
func WithBlobPlatform(platform *ocispec.Platform) BlobOption {
	return func(blob *Blob) error {
		blob.desc.Platform = platform
		return nil
	}
}

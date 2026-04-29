// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import (
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// IsOCIIndexKraftKitCompatible is a utility method that is used to determine
// whether the provided OCI Specification Index structure is compatible with
// KraftKit.
func IsOCIIndexKraftKitCompatible(index *ocispec.Index) (bool, error) {
	if index == nil {
		return false, fmt.Errorf("provided index is nil")
	}

	return true, nil
}

// IsOCIManifestKraftKitCompatible is a utility method that is used to determine
// whether the provided OCI Specification Manifest structure is compatible with
// KraftKit.
func IsOCIManifestKraftKitCompatible(manifest *ocispec.Manifest) (bool, error) {
	if manifest == nil {
		return false, fmt.Errorf("provided manifest is nil")
	}

	return true, nil
}

// IsOCIDescriptorKraftKitCompatible is a utility method that is used to
// determine whether the provided OCI Specification Descriptor structure is
// compatible with KraftKit.
func IsOCIDescriptorKraftKitCompatible(descriptor *ocispec.Descriptor) (bool, error) {
	if descriptor == nil {
		return false, fmt.Errorf("provided descriptor is nil")
	}

	return true, nil
}

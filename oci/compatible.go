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
// KraftKit.  Ultimately, this is achieved by testing whether the annotation
// "sh.kraftkit.version" has been set.  The value of this annotation is
// discarded.
func IsOCIIndexKraftKitCompatible(index *ocispec.Index) (bool, error) {
	if index == nil {
		return false, fmt.Errorf("provided index is nil")
	}

	if index.Annotations == nil {
		return false, fmt.Errorf("index does not contain any annotations")
	}

	if _, ok := index.Annotations[AnnotationKraftKitVersion]; !ok {
		return false, fmt.Errorf("index does not contain '%s' annotation", AnnotationKraftKitVersion)
	}

	return true, nil
}

// IsOCIManifestKraftKitCompatible is a utility method that is used to determine
// whether the provided OCI Specification Manifest structure is compatible with
// KraftKit.  Ultimately, this is achieved by testing whether the annotation
// "sh.kraftkit.version" has been set.  The value of this annotation is
// discarded.
func IsOCIManifestKraftKitCompatible(manifest *ocispec.Manifest) (bool, error) {
	if manifest == nil {
		return false, fmt.Errorf("provided manifest is nil")
	}

	if manifest.Annotations == nil {
		return false, fmt.Errorf("manifest does not contain any annotations")
	}

	if _, ok := manifest.Annotations[AnnotationKraftKitVersion]; !ok {
		return false, fmt.Errorf("manifest does not contain '%s' annotation", AnnotationKraftKitVersion)
	}

	return true, nil
}

// IsOCIDescriptorKraftKitCompatible is a utility method that is used to
// determine whether the provided OCI Specification Descriptor structure is
// compatible with KraftKit.  Ultimately, this is achieved by testing whether
// the annotation "sh.kraftkit.version" has been set.  The value of this
// annotation is discarded.
func IsOCIDescriptorKraftKitCompatible(descriptor *ocispec.Descriptor) (bool, error) {
	if descriptor == nil {
		return false, fmt.Errorf("provided descriptor is nil")
	}

	if descriptor.Annotations == nil {
		return false, fmt.Errorf("descriptor does not contain any annotations")
	}

	if _, ok := descriptor.Annotations[AnnotationKraftKitVersion]; !ok {
		return false, fmt.Errorf("descriptor does not contain '%s' annotation", AnnotationKraftKitVersion)
	}

	return true, nil
}

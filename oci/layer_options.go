// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import "fmt"

type LayerOption func(*Layer) error

// WithLayerAnnotation sets an annotation for a particular layer
func WithLayerAnnotation(key, val string) LayerOption {
	return func(layer *Layer) error {
		if layer.blob == nil {
			return fmt.Errorf("cannot apply layer annotation without creating blob")
		}

		if layer.blob.desc.Annotations == nil {
			layer.blob.desc.Annotations = make(map[string]string)
		}

		layer.blob.desc.Annotations[key] = val

		return nil
	}
}

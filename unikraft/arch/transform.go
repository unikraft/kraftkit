// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package arch

import (
	"context"
	"fmt"

	"kraftkit.sh/unikraft"
)

// TransformFromSchema parses an input schema and returns an instantiated
// ArchitectureConfig
func TransformFromSchema(ctx context.Context, data interface{}) (interface{}, error) {
	uk := unikraft.FromContext(ctx)
	architecture := ArchitectureConfig{}

	switch value := data.(type) {
	case string:
		architecture.name = value
	default:
		return nil, fmt.Errorf("invalid type %T for architecture", data)
	}

	if uk != nil && uk.UK_BASE != "" {
		architecture.path, _ = unikraft.PlaceComponent(
			uk.UK_BASE,
			unikraft.ComponentTypeArch,
			architecture.name,
		)
	}

	return architecture, nil
}

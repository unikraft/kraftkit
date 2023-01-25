// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package plat

import (
	"context"
	"fmt"

	"kraftkit.sh/unikraft"
)

// TransformFromSchema parses an input schema and returns an instantiated
// PlatformConfig
func TransformFromSchema(ctx context.Context, data interface{}) (interface{}, error) {
	uk := unikraft.FromContext(ctx)
	platform := PlatformConfig{}

	switch value := data.(type) {
	case string:
		platform.name = value
	default:
		return nil, fmt.Errorf("invalid type %T for platform", data)
	}

	if uk != nil && uk.UK_BASE != "" {
		platform.path, _ = unikraft.PlaceComponent(
			uk.UK_BASE,
			unikraft.ComponentTypePlat,
			platform.name,
		)
	}

	return platform, nil
}

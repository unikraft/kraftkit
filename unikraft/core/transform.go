// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package core

import (
	"context"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

// TransformFromSchema parses an input schema and returns an instantiated
// UnikraftConfig
func TransformFromSchema(ctx context.Context, props interface{}) (interface{}, error) {
	uk := unikraft.FromContext(ctx)
	core := UnikraftConfig{}

	if uk != nil && uk.UK_BASE != "" {
		core.path, _ = unikraft.PlaceComponent(
			uk.UK_BASE,
			unikraft.ComponentTypeCore,
			"unikraft",
		)
	}

	c, err := component.TranslateFromSchema(props)
	if err != nil {
		return nil, err
	}

	if source, ok := c["source"]; ok {
		core.source = source.(string)
	}

	if version, ok := c["version"]; ok {
		core.version = version.(string)
	}

	if kconf, ok := c["kconfig"]; ok {
		core.kconfig = kconf.(kconfig.KeyValueMap)
	}

	return core, nil
}

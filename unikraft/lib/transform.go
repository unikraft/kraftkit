// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"context"
	"fmt"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

// TransformFromSchema parses an input schema and returns an instantiated
// LibraryConfig
func TransformFromSchema(ctx context.Context, name string, props interface{}) (LibraryConfig, error) {
	uk := unikraft.FromContext(ctx)
	lib := LibraryConfig{}

	if len(name) > 0 {
		lib.name = name
	}

	if uk != nil && uk.UK_BASE != "" {
		lib.path, _ = unikraft.PlaceComponent(
			uk.UK_BASE,
			unikraft.ComponentTypeLib,
			lib.name,
		)
	}

	c, err := component.TranslateFromSchema(props)
	if err != nil {
		return lib, err
	}

	if source, ok := c["source"]; ok {
		lib.source = source.(string)
	}

	if version, ok := c["version"]; ok {
		lib.version = version.(string)
	}

	if kconf, ok := c["kconfig"]; ok {
		lib.kconfig = kconf.(kconfig.KeyValueMap)
	}

	return lib, nil
}

// TransformMapFromSchema
func TransformMapFromSchema(ctx context.Context, data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case map[string]interface{}:
		libraries := make(map[string]interface{})
		for name, props := range value {
			switch props.(type) {
			case string, map[string]interface{}:
				comp, err := TransformFromSchema(ctx, name, props)
				if err != nil {
					return nil, err
				}
				libraries[name] = comp

			default:
				return data, fmt.Errorf("invalid type %T for component", props)
			}
		}

		return libraries, nil
	}

	return data, fmt.Errorf("invalid type %T for library", data)
}

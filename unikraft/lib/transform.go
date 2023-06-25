// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"context"
	"fmt"
	"os"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/utils"
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

		// If the provided source is a directory on the host, set the "path" to this
		// value.  The "path" is the location on disk where the microlibrary will
		// eventually saved by the relevant package manager.  For completeness, use
		// absolute paths for both the path and the source.
		if f, err := os.Stat(lib.source); err == nil && f.IsDir() {
			if uk != nil && uk.UK_BASE != "" {
				lib.path = utils.RelativePath(uk.UK_BASE, lib.source)
				lib.source = lib.path
			} else {
				lib.path = lib.source
			}
		}
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

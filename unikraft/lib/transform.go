// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
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

	switch entry := props.(type) {
	case string:
		if strings.Contains(entry, "@") {
			split := strings.Split(entry, "@")
			if len(split) == 2 {
				lib.source = split[0]
				lib.version = split[1]
			}
		} else if f, err := os.Stat(entry); err == nil && f.IsDir() {
			lib.source = entry
		} else if u, err := url.Parse(entry); err == nil && u.Scheme != "" && u.Host != "" {
			lib.source = u.Path
		} else {
			lib.version = entry
		}

	case map[string]interface{}:
		for key, prop := range entry {
			switch key {
			case "version":
				lib.version = prop.(string)

			case "source":
				prop := prop.(string)
				if strings.Contains(prop, "@") {
					split := strings.Split(prop, "@")
					if len(split) == 2 {
						lib.version = split[1]
						prop = split[0]
					}
				}

				lib.source = prop

			case "kconfig":
				switch tprop := prop.(type) {
				case map[string]interface{}:
					lib.kconfig = kconfig.NewKeyValueMapFromMap(tprop)
				case []interface{}:
					lib.kconfig = kconfig.NewKeyValueMapFromSlice(tprop...)
				}
			}
		}
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

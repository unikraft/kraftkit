// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package core

import (
	"context"
	"net/url"
	"os"
	"strings"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
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

	switch entry := props.(type) {
	case string:
		if strings.Contains(entry, "@") {
			split := strings.Split(entry, "@")
			if len(split) == 2 {
				core.source = split[0]
				core.version = split[1]
			}
		} else if f, err := os.Stat(entry); err == nil && f.IsDir() {
			core.source = entry
		} else if u, err := url.Parse(entry); err == nil && u.Scheme != "" && u.Host != "" {
			core.source = u.Path
		} else {
			core.version = entry
		}

	case map[string]interface{}:
		for key, prop := range entry {
			switch key {
			case "version":
				core.version = prop.(string)

			case "source":
				prop := prop.(string)
				if strings.Contains(prop, "@") {
					split := strings.Split(prop, "@")
					if len(split) == 2 {
						core.version = split[1]
						prop = split[0]
					}
				}

				core.source = prop

			case "kconfig":
				switch tprop := prop.(type) {
				case map[string]interface{}:
					core.kconfig = kconfig.NewKeyValueMapFromMap(tprop)
				case []interface{}:
					core.kconfig = kconfig.NewKeyValueMapFromSlice(tprop...)
				}
			}
		}
	}

	return core, nil
}

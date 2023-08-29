// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package elfloader

import (
	"context"
	"fmt"
	"strings"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft/component"
)

// TransformFromSchema parses an input schema and returns an instantiated
// ELFLoader.
func TransformFromSchema(ctx context.Context, props interface{}) (interface{}, error) {
	elfloader := ELFLoader{}

	switch entry := props.(type) {
	case string:
		var split []string
		// Is there a schema specifier?
		if strings.Contains(entry, "://") {
			split = strings.Split(entry, "://")
			switch split[0] {
			case "oci":
				split = strings.Split(split[1], ":")
				elfloader.source = split[0]
				if len(split) > 1 {
					elfloader.version = split[1]
				}
			case "kernel":
				elfloader.kernel = split[1]
			}
		} else {
			// The following sequence parses the format:
			split = strings.Split(entry, ":")
			if len(split) > 2 {
				return nil, fmt.Errorf("expected format template value to be <oci>:<tag>")
			}
			elfloader.name = split[0]
			if len(split) > 1 {
				elfloader.version = split[1]
			}
		}

	case map[string]interface{}:
		c, err := component.TranslateFromSchema(props)
		if err != nil {
			return nil, err
		}

		if source, ok := c["source"]; ok {
			elfloader.source = source.(string)
		}

		if version, ok := c["version"]; ok {
			elfloader.version = version.(string)
		}

		if kconf, ok := c["kconfig"]; ok {
			elfloader.kconfig = kconf.(kconfig.KeyValueMap)
		}
	}

	return elfloader, nil
}

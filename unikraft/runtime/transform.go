// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package runtime

import (
	"context"
	"fmt"
	"strings"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft/component"
)

// TransformFromSchema parses an input schema and returns an instantiated
// runtime.
func TransformFromSchema(ctx context.Context, props interface{}) (interface{}, error) {
	runtime := Runtime{}

	switch entry := props.(type) {
	case string:
		var split []string
		// Is there a schema specifier?
		if strings.Contains(entry, "://") {
			split = strings.Split(entry, "://")
			switch split[0] {
			case "oci":
				split = strings.Split(split[1], ":")
				runtime.source = split[0]
				if len(split) > 1 {
					runtime.version = split[1]
				}
			case "kernel":
				runtime.kernel = split[1]
			}
		} else {
			// The following sequence parses the format:
			split = strings.Split(entry, ":")
			if len(split) > 2 {
				return nil, fmt.Errorf("expected format template value to be <oci>:<tag>")
			}
			runtime.name = split[0]
			if len(split) > 1 {
				runtime.version = split[1]
			}
		}

	case map[string]interface{}:
		c, err := component.TranslateFromSchema(props)
		if err != nil {
			return nil, err
		}

		if source, ok := c["source"]; ok {
			runtime.source, ok = source.(string)
			if !ok {
				return nil, fmt.Errorf("runtime 'source' must be a string, got %T", source)
			}
		}

		if version, ok := c["version"]; ok {
			runtime.version, ok = version.(string)
			if !ok {
				return nil, fmt.Errorf("runtime 'version' must be a string, got %T", version)
			}
		}

		if kconf, ok := c["kconfig"]; ok {
			runtime.kconfig, ok = kconf.(kconfig.KeyValueMap)
			if !ok {
				return nil, fmt.Errorf("runtime 'kconfig' must be a mapping, got %T", kconf)
			}
		}
	}

	return runtime, nil
}

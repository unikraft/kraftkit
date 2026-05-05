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
		// Is there a schema specifier?
		if strings.Contains(entry, "://") {
			split := strings.SplitN(entry, "://", 2)
			switch split[0] {
			case "oci":
				runtime.name, runtime.version = splitRuntimeNameVersion(split[1])
				runtime.source = runtime.QueryName()
			case "kernel":
				runtime.kernel = split[1]
			}
		} else {
			runtime.name, runtime.version = splitRuntimeNameVersion(entry)
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

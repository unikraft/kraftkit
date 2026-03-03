// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package volume

import (
	"context"
	"fmt"
	"strings"
)

// TransformFromSchema parses an input schema and returns an instantiated
// VolumeConfig
func TransformFromSchema(ctx context.Context, data interface{}) (interface{}, error) {
	volume := VolumeConfig{}

	switch entry := data.(type) {
	case string:
		var split []string
		if strings.Contains(entry, ":") {
			split = strings.Split(entry, ":")
			if len(split) > 2 {
				return nil, fmt.Errorf("expected format template value to be <url>@<version>")
			}

			volume.source = split[0]
			if len(split) == 2 {
				volume.destination = split[1]
			}
		} else {
			// When no colon is specified, assume the root file system
			volume.source = entry
			volume.destination = "/"
		}

	case map[string]interface{}:
		for key, prop := range entry {
			switch key {
			case "driver":
				driverStr, ok := prop.(string)
				if !ok {
					return nil, fmt.Errorf("malformed Kraftfile: 'volumes.driver' must be a string, got %T", prop)
				}
				volume.driver = driverStr

			case "source":
				sourceStr, ok := prop.(string)
				if !ok {
					return nil, fmt.Errorf("malformed Kraftfile: 'volumes.source' must be a string, got %T", prop)
				}
				volume.source = sourceStr

			case "destination":
				destStr, ok := prop.(string)
				if !ok {
					return nil, fmt.Errorf("malformed Kraftfile: 'volumes.destination' must be a string, got %T", prop)
				}
				volume.destination = destStr

			case "readonly":
				readOnlyBool, ok := prop.(bool)
				if !ok {
					return nil, fmt.Errorf("malformed Kraftfile: 'volumes.readonly' must be a boolean, got %T", prop)
				}
				volume.readOnly = readOnlyBool

			}
		}
	}

	return volume, nil
}

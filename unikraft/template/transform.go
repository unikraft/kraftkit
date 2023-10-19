// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package template

import (
	"context"
	"os"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/utils"
)

// TransformFromSchema parses an input schema and returns an instantiated
// TemplateConfig
func TransformFromSchema(ctx context.Context, props interface{}) (interface{}, error) {
	uk := unikraft.FromContext(ctx)
	template := TemplateConfig{}

	c, err := component.TranslateFromSchema(props)
	if err != nil {
		return template, err
	}

	if name, ok := c["name"].(string); ok && uk != nil && uk.UK_BASE != "" {
		template.path, _ = unikraft.PlaceComponent(
			uk.UK_BASE,
			unikraft.ComponentTypeApp,
			name,
		)
	}

	if source, ok := c["source"]; ok {
		template.source = source.(string)

		// If the provided source is a directory on the host, set the "path" to this
		// value.  The "path" is the location on disk where the microlibrary will
		// eventually saved by the relevant package manager.  For completeness, use
		// absolute paths for both the path and the source.
		if f, err := os.Stat(template.source); err == nil && f.IsDir() {
			if uk != nil && uk.UK_BASE != "" {
				template.path = utils.RelativePath(uk.UK_BASE, template.source)
				template.source = template.path
			} else {
				template.path = template.source
			}
		}
	}

	if name, ok := c["name"]; ok {
		template.name = name.(string)
	}

	if version, ok := c["version"]; ok {
		template.version = version.(string)
	}

	if kconf, ok := c["kconfig"]; ok {
		template.kconfig = kconf.(kconfig.KeyValueMap)
	}

	return template, nil
}

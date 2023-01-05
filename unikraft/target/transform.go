// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package target

import (
	"context"
	"fmt"
	"path/filepath"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
)

func TransformFromSchema(ctx context.Context, data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case map[string]interface{}:
		uk := unikraft.FromContext(ctx)
		t := TargetConfig{}

		if uk != nil && uk.UK_NAME != "" {
			t.name = uk.UK_NAME
		}

		for key, prop := range value {
			switch key {
			case "name":
				t.name = prop.(string)

			case "architecture", "arch":
				architecture, err := arch.TransformFromSchema(ctx, prop)
				if err != nil {
					return nil, err
				}

				t.architecture = architecture.(arch.ArchitectureConfig)

			case "platform", "plat":
				platform, err := plat.TransformFromSchema(ctx, prop)
				if err != nil {
					return nil, err
				}

				t.platform = platform.(plat.PlatformConfig)

			case "kernel":
				t.name = prop.(string)

			case "kconfig":
				switch tprop := prop.(type) {
				case map[string]interface{}:
					t.kconfig = kconfig.NewKeyValueMapFromMap(tprop)
				case []interface{}:
					t.kconfig = kconfig.NewKeyValueMapFromSlice(tprop...)
				}
			}
		}

		if uk != nil && uk.BUILD_DIR != "" {
			if t.kernel == "" {
				kernel, err := KernelName(t)
				if err != nil {
					return nil, err
				}

				t.kernel = filepath.Join(uk.BUILD_DIR, kernel)
			}

			if t.kernelDbg == "" {
				kernelDbg, err := KernelDbgName(t)
				if err != nil {
					return nil, err
				}

				t.kernelDbg = filepath.Join(uk.BUILD_DIR, kernelDbg)
			}
		}

		return t, nil
	}

	return data, fmt.Errorf("invalid type %T for target", data)
}

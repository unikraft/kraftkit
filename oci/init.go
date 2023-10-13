// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import (
	"kraftkit.sh/config"
	"kraftkit.sh/packmanager"
)

func RegisterPackageManager(cfg *config.KraftKit) func(u *packmanager.UmbrellaManager) error {
	return func(u *packmanager.UmbrellaManager) error {
		return u.RegisterPackageManager(
			OCIFormat,
			NewOCIManager,
			WithDefaultAuth(cfg),
			WithDefaultRegistries(cfg),
			WithDetectHandler(cfg),
		)
	}
}

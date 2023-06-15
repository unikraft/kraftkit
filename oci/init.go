// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import (
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/packmanager"
)

// FIXME(antoineco): avoid init, initialize things where needed
func init() {
	// Register a new pkg.Package type
	_ = packmanager.RegisterPackageManager(
		OCIFormat,
		NewOCIManager,
		WithDefaultAuth(),
		WithDefaultRegistries(),
		WithDetectHandler(),
	)

	// Register additional command-line flags
	cmdfactory.RegisterFlag(
		"kraft pkg",
		cmdfactory.StringVar(
			&flagTag,
			"oci-tag",
			"",
			"Set the OCI image tag.",
		),
	)
	cmdfactory.RegisterFlag(
		"kraft pkg",
		cmdfactory.BoolVar(
			&flagUseMediaTypes,
			"oci-use-media-types",
			false,
			"Use media types as opposed to well-known paths (experimental).",
		),
	)
}

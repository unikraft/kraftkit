// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package image handles setting metadata on resulting container images.
package image

import (
	"runtime"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"kraftkit.sh/oci"
)

// UnikraftImageConfig provided metadata enables running unikraft images through docker run.
func UnikraftImageConfig() *specs.Image {
	return &specs.Image{
		Platform: specs.Platform{
			Architecture: runtime.GOARCH,
			OS:           runtime.GOOS,
		},
		RootFS: specs.RootFS{
			Type: "layers",
		},
		Config: specs.ImageConfig{
			WorkingDir: "/",
			Entrypoint: []string{oci.WellKnownKernelPath},
		},
	}
}

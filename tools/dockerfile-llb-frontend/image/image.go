// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package image handles setting metadata on resulting container images.
package image

import (
	"github.com/moby/buildkit/util/system"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

func emptyImage() *specs.Image {
	image := &specs.Image{}
	image.RootFS.Type = "layers"
	image.Config.WorkingDir = "/"
	image.Config.Env = []string{"PATH=" + system.DefaultPathEnv("linux")}
	return image
}

// NewImageConfig returns empty container image metadata.
func NewImageConfig() *specs.Image {
	// (todo): We might decide what else to set here once we get to the run part.
	return emptyImage()
}

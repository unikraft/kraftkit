// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package pkg

import (
	"context"
	"fmt"
	"strings"

	"kraftkit.sh/pack"
)

type packagerDockerfile struct{}

// String implements fmt.Stringer.
func (p *packagerDockerfile) String() string {
	return "dockerfile"
}

// Packagable implements packager.
func (p *packagerDockerfile) Packagable(ctx context.Context, opts *PkgOptions, args ...string) (bool, error) {
	if opts.Project == nil {
		// Do not capture the the project is not initialized, as we can still build
		// the unikernel using the Dockerfile provided with the `--rootfs`.
		_ = opts.initProject(ctx)
	}

	if opts.Project != nil && opts.Project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.Project.Rootfs()
	}

	// TODO(nderjung): This is a very naiive check and should be improved,
	// potentially using an external library which parses the Dockerfile syntax.
	// In most cases, however, the Dockerfile is usually named `Dockerfile`.
	if !strings.Contains(strings.ToLower(opts.Rootfs), "dockerfile") {
		return false, fmt.Errorf("%s is not a Dockerfile", opts.Rootfs)
	}

	return true, nil
}

// Build implements packager.
func (p *packagerDockerfile) Pack(ctx context.Context, opts *PkgOptions, args ...string) ([]pack.Package, error) {
	return (&packagerKraftfileRuntime{}).Pack(ctx, opts, args...)
}

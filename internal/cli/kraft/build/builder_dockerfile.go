// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"fmt"
	"strings"
)

// builderDockerfile is a builder that only uses a `Dockerfile` to build a
// unikernel.  In this context, no `Kraftfile` is present.
type builderDockerfile struct{}

// String implements fmt.Stringer.
func (build *builderDockerfile) String() string {
	return "dockerfile"
}

// Buildable implements builder.
func (build *builderDockerfile) Buildable(ctx context.Context, opts *BuildOptions, args ...string) (bool, error) {
	if opts.NoRootfs {
		return false, fmt.Errorf("building rootfs disabled")
	}

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
		return false, fmt.Errorf("file is not a Dockerfile")
	}

	return true, nil
}

// Prepare implements builder.
func (*builderDockerfile) Prepare(ctx context.Context, opts *BuildOptions, _ ...string) (err error) {
	return (&builderKraftfileRuntime{}).Prepare(ctx, opts)
}

// Build implements builder.
func (*builderDockerfile) Build(_ context.Context, _ *BuildOptions, _ ...string) error {
	return nil
}

// Statistics implements builder.
func (*builderDockerfile) Statistics(ctx context.Context, opts *BuildOptions, args ...string) error {
	return fmt.Errorf("cannot calculate statistics of pre-built unikernel runtime")
}

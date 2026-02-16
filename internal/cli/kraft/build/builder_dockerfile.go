// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"kraftkit.sh/log"
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
		if err := opts.initProject(ctx); err != nil {
			log.G(ctx).WithError(err).Warn("could not initialize project")
		}
	}

	if opts.Project != nil && opts.Project.Rootfs() != "" && opts.Rootfs == "" {
		opts.Rootfs = opts.Project.Rootfs()
	}

	if opts.Project != nil && opts.Project.InitrdFsType().String() != "" && opts.RootfsType == "" {
		opts.RootfsType = opts.Project.InitrdFsType()
	}

	// Dockerfile validation using buildkit parser
	if !isDockerfile(ctx, opts.Rootfs) {
		return false, fmt.Errorf("file is not a Dockerfile")
	}

	return true, nil
}

func isDockerfile(ctx context.Context, path string) bool {

	base := strings.ToLower(filepath.Base(path))

	looksLikeDockerfile := strings.Contains(base, "dockerfile") ||
		strings.HasSuffix(base, ".docker")

	valid, err := isValidDockerfile(path)

	if err != nil {
		// Log the error instead of silently discarding it
		log.G(ctx).Debugf("error validating Dockerfile %s: %v", path, err)

		return false
	}

	if looksLikeDockerfile && !valid {
		log.G(ctx).Warnf("file %s looks like a Dockerfile but has invalid syntax", path)
	}

	return valid
}

// isValidDockerfile parses the file to check if it's a valid Dockerfile
func isValidDockerfile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Parse the Dockerfile using buildkit parser
	result, err := parser.Parse(f)
	if err != nil {
		// Parsing failed then it's not a valid Dockerfile
		return false, nil
	}

	// Check if it has valid AST with at least one instruction
	if result == nil || result.AST == nil || len(result.AST.Children) == 0 {
		return false, nil
	}

	// A valid Dockerfile MUST have a FROM instruction
	// (or ARG before FROM, which is also valid)
	hasFrom := false
	for _, child := range result.AST.Children {
		if child != nil && strings.ToUpper(child.Value) == "FROM" {
			hasFrom = true
			break
		}
	}

	return hasFrom, nil
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

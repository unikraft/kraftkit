// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"fmt"
	"strings"

	"kraftkit.sh/internal/cli/kraft/pkg"
	"kraftkit.sh/pack"
)

type deployerKraftfileRuntime struct{}

func (deployer *deployerKraftfileRuntime) String() string {
	return "kraftfile-runtime"
}

func (deployer *deployerKraftfileRuntime) Deployable(ctx context.Context, opts *DeployOptions, args ...string) (bool, error) {
	if opts.Project == nil {
		if err := opts.initProject(ctx); err != nil {
			return false, err
		}
	}

	if opts.Project.Unikraft(ctx) != nil {
		return false, nil
	}

	if opts.Project.Runtime() == nil {
		return false, fmt.Errorf("cannot package without runtime specification")
	}

	return true, nil
}

func (deployer *deployerKraftfileRuntime) Prepare(ctx context.Context, opts *DeployOptions, args ...string) ([]pack.Package, error) {
	if !strings.HasPrefix(opts.Name, "index.unikraft.io") {
		opts.Name = fmt.Sprintf("index.unikraft.io/%s/%s:latest", opts.Auth.User, opts.Name)
	}

	return pkg.Pkg(ctx, &pkg.PkgOptions{
		Format:    "oci",
		Kraftfile: opts.Kraftfile,
		Name:      opts.Name,
		Push:      true,
		Strategy:  opts.Strategy,
		Workdir:   opts.Workdir,
	})
}

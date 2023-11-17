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

	if strings.HasPrefix(opts.Project.Runtime().Name(), "unikraft.io") {
		opts.Project.Runtime().SetName("index." + opts.Project.Runtime().Name())
	}
	if strings.HasPrefix(opts.Project.Runtime().Name(), opts.Auth.User) {
		opts.Project.Runtime().SetName("index.unikraft.io/" + opts.Project.Runtime().Name())
	}
	if !strings.HasPrefix(opts.Project.Runtime().Name(), "index.unikraft.io") {
		opts.Project.Runtime().SetName("index.unikraft.io/official/" + opts.Project.Runtime().Name())
	}

	return true, nil
}

func (deployer *deployerKraftfileRuntime) Prepare(ctx context.Context, opts *DeployOptions, args ...string) ([]pack.Package, error) {
	return pkg.Pkg(ctx, &pkg.PkgOptions{
		Architecture: "x86_64",
		Format:       "oci",
		Kraftfile:    opts.Kraftfile,
		Name:         args[0],
		Platform:     "kraftcloud",
		Project:      opts.Project,
		Push:         true,
		Strategy:     opts.Strategy,
		Workdir:      opts.Workdir,
	})
}

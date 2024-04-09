// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package pkg

import (
	"context"
	"os"
	"strings"

	"kraftkit.sh/unikraft/app"
)

// initProject sets up the project based on the provided context and
// options.
func (opts *PkgOptions) initProject(ctx context.Context) error {
	var err error

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.Workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Interpret the project directory
	opts.Project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil {
		return err
	}

	return nil
}

// aggregateEnvs aggregates the environment variables from the project and
// the cli options, filling in missing values with the host environment.
func (opts *PkgOptions) aggregateEnvs() []string {
	envs := make(map[string]string)

	if opts.Project.Env() != nil {
		envs = opts.Project.Env()
	}

	// Add the cli environment
	for _, env := range opts.Env {
		if strings.ContainsRune(env, '=') {
			parts := strings.SplitN(env, "=", 2)
			envs[parts[0]] = parts[1]
			continue
		}

		envs[env] = os.Getenv(env)
	}

	// Aggregate all the environment variables
	var env []string
	for k, v := range envs {
		env = append(env, k+"="+v)
	}

	return env
}

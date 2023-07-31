// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/exec"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

func (opts *GithubAction) build(ctx context.Context) error {
	// Filter project targets by any provided input arguments
	targets := target.Filter(
		opts.project.Targets(),
		opts.Arch,
		opts.Plat,
		opts.Target,
	)

	if len(targets) != 1 {
		// TODO(nderjung): We should support building multiple targets in the
		// future, but for now we disable this ability.  This is largely to do with
		// package management afterwards which does not yet support multi-target
		// artifacts.  Once this is supported, we can enable multiple target-builds
		// (and packaging).  Moreover, since it is possible to also execute the
		// unikernel after a successful build via this action, multiple targets
		// would also fail at this step.
		return fmt.Errorf("cannot build more than one target using action")
	}

	if err := opts.project.Configure(
		ctx,
		targets[0], // Target-specific options
		nil,        // No extra configuration options
		make.WithSilent(true),
		make.WithExecOptions(
			exec.WithStdin(iostreams.G(ctx).In),
			exec.WithStdout(log.G(ctx).Writer()),
			exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
		),
	); err != nil {
		return fmt.Errorf("could not configure project: %w", err)
	}

	return opts.project.Build(
		ctx,
		targets[0], // Target-specific options
		app.WithBuildMakeOptions(
			make.WithMaxJobs(true),
			make.WithExecOptions(
				exec.WithStdout(log.G(ctx).Writer()),
				exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
			),
		),
	)
}

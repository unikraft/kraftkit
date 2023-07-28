// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"fmt"
	"strings"

	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/target"
)

// pack
func (opts *GithubAction) packAndPush(ctx context.Context) error {
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

	output := opts.Output
	var format pack.PackageFormat
	if strings.Contains(opts.Output, "://") {
		split := strings.SplitN(opts.Output, "://", 2)
		format = pack.PackageFormat(split[0])
		output = split[1]
	} else {
		format = targets[0].Format()
	}

	var err error
	pm := packmanager.G(ctx)

	// Switch the package manager the desired format for this target
	if format != "auto" {
		pm, err = pm.From(format)
		if err != nil {
			return err
		}
	}

	popts := []packmanager.PackOption{
		packmanager.PackInitrd(opts.InitRd),
		packmanager.PackKConfig(opts.Kconfig),
		packmanager.PackOutput(output),
	}

	if ukversion, ok := targets[0].KConfig().Get(unikraft.UK_FULLVERSION); ok {
		popts = append(popts,
			packmanager.PackWithKernelVersion(ukversion.Value),
		)
	}

	packs, err := pm.Pack(ctx, targets[0], popts...)
	if err != nil {
		return err
	}

	if opts.Push {
		return packs[0].Push(ctx)
	}

	return nil
}

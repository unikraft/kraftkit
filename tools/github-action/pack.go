// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package main

import (
	"context"
	"strings"

	"github.com/mattn/go-shellwords"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
)

// pack
func (opts *GithubAction) packAndPush(ctx context.Context) error {
	output := opts.Output
	var format pack.PackageFormat
	if strings.Contains(opts.Output, "://") {
		split := strings.SplitN(opts.Output, "://", 2)
		format = pack.PackageFormat(split[0])
		output = split[1]
	} else {
		format = "oci"
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

	// If no arguments have been specified, use the ones which are default and
	// that have been included in the package.
	if len(opts.Args) == 0 {
		if len(opts.project.Command()) > 0 {
			opts.Args = opts.project.Command()
		} else if len(opts.target.Command()) > 0 {
			opts.Args = opts.target.Command()
		}
	}

	cmdShellArgs, err := shellwords.Parse(strings.Join(opts.Args, " "))
	if err != nil {
		return err
	}

	popts := []packmanager.PackOption{
		packmanager.PackInitrd(opts.initrdPath),
		packmanager.PackKConfig(true),
		packmanager.PackName(output),
		packmanager.PackMergeStrategy(packmanager.MergeStrategy(opts.Strategy)),
		packmanager.PackArgs(cmdShellArgs...),
	}

	if ukversion, ok := opts.target.KConfig().Get(unikraft.UK_FULLVERSION); ok {
		popts = append(popts,
			packmanager.PackWithKernelVersion(ukversion.Value),
		)
	}

	packs, err := pm.Pack(ctx, opts.target, popts...)
	if err != nil {
		return err
	}

	if opts.Push {
		return packs[0].Push(ctx)
	}

	return nil
}

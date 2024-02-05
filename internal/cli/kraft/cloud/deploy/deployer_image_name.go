// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"fmt"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/processtree"
	kraftcloudinstances "sdk.kraft.cloud/instances"
)

type deployerImageName struct{}

func (deployer *deployerImageName) String() string {
	return "image-name"
}

func (deployer *deployerImageName) Deployable(ctx context.Context, opts *DeployOptions, args ...string) (bool, error) {
	if err := opts.initProject(ctx); err != nil && len(args) > 0 {
		return true, nil
	}

	return false, fmt.Errorf("context contains project")
}

func (deployer *deployerImageName) Deploy(ctx context.Context, opts *DeployOptions, args ...string) ([]kraftcloudinstances.Instance, error) {
	var err error
	var instance *kraftcloudinstances.Instance

	paramodel, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithRenderer(
				log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
			),
			processtree.WithFailFast(true),
			processtree.WithHideOnSuccess(true),
			processtree.WithTimeout(opts.Timeout),
		},
		processtree.NewProcessTreeItem(
			"deploying",
			"",
			func(ctx context.Context) error {
				instance, err = create.Create(ctx, &create.CreateOptions{
					Env:                    opts.Env,
					Features:               opts.Features,
					FQDN:                   opts.FQDN,
					Image:                  args[0],
					Memory:                 opts.Memory,
					Metro:                  opts.Metro,
					Name:                   opts.Name,
					Ports:                  opts.Ports,
					Replicas:               opts.Replicas,
					ScaleToZero:            opts.ScaleToZero,
					ServiceGroupNameOrUUID: opts.ServiceGroupNameOrUUID,
					Start:                  !opts.NoStart,
					SubDomain:              opts.SubDomain,
				}, args[1:]...)
				if err != nil {
					return fmt.Errorf("could not create instance: %w", err)
				}

				return nil
			},
		),
	)
	if err != nil {
		return nil, err
	}

	if err := paramodel.Start(); err != nil {
		return nil, err
	}

	return []kraftcloudinstances.Instance{*instance}, nil
}

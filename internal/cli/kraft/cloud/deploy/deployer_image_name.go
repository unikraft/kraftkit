// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"fmt"
	"strings"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	"kraftkit.sh/log"
	"kraftkit.sh/oci"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
	kraftcloudinstances "sdk.kraft.cloud/instances"
)

type deployerImageName struct {
	imageName string
	args      []string
}

func (deployer *deployerImageName) String() string {
	if len(deployer.args) == 0 {
		return fmt.Sprintf("run the '%s' image and ignore cwd", deployer.imageName)
	}

	return fmt.Sprintf("run the '%s' image, use '%s' as arg(s) and ignore cwd", deployer.imageName, strings.Join(deployer.args, " "))
}

func (deployer *deployerImageName) Name() string {
	return "image-name"
}

func (deployer *deployerImageName) Deployable(ctx context.Context, opts *DeployOptions, args ...string) (bool, error) {
	if len(args) == 0 {
		return false, fmt.Errorf("no image specified")
	}

	pm, err := packmanager.G(ctx).From(oci.OCIFormat)
	if err != nil {
		return false, fmt.Errorf("getting oci package manager: %w", err)
	}

	imageName := args[0]

	if strings.HasPrefix(imageName, "unikraft.io") {
		imageName = "index." + imageName
	} else if strings.Contains(imageName, "/") && !strings.Contains(imageName, "unikraft.io") {
		imageName = "index.unikraft.io/" + imageName
	} else if !strings.HasPrefix(imageName, "index.unikraft.io") {
		imageName = "index.unikraft.io/official/" + imageName
	}

	if _, compatible, err := pm.IsCompatible(ctx, imageName,
		packmanager.WithRemote(true),
	); !compatible {
		return false, err
	}

	deployer.imageName = args[0]
	deployer.args = args[1:]

	return true, nil
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
					Image:                  deployer.imageName,
					Memory:                 opts.Memory,
					Metro:                  opts.Metro,
					Name:                   opts.Name,
					Ports:                  opts.Ports,
					Replicas:               opts.Replicas,
					ScaleToZero:            opts.ScaleToZero,
					ServiceGroupNameOrUUID: opts.ServiceGroupNameOrUUID,
					Start:                  !opts.NoStart,
					SubDomain:              opts.SubDomain,
					Token:                  opts.Token,
				}, deployer.args...)
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

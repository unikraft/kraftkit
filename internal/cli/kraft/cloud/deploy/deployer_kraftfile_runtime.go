// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	"kraftkit.sh/internal/cli/kraft/pkg"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/processtree"

	kraftcloudinstances "sdk.kraft.cloud/instances"
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

	if opts.Runtime != "" {
		opts.Project.Runtime().SetName(opts.Runtime)
	}
	if strings.HasPrefix(opts.Project.Runtime().Name(), "unikraft.io") {
		opts.Project.Runtime().SetName("index." + opts.Project.Runtime().Name())
	} else if strings.Contains(opts.Project.Runtime().Name(), "/") && !strings.Contains(opts.Project.Runtime().Name(), "unikraft.io") {
		opts.Project.Runtime().SetName("index.unikraft.io/" + opts.Project.Runtime().Name())
	} else if !strings.HasPrefix(opts.Project.Runtime().Name(), "index.unikraft.io") {
		opts.Project.Runtime().SetName("index.unikraft.io/official/" + opts.Project.Runtime().Name())
	}

	return true, nil
}

func (deployer *deployerKraftfileRuntime) Deploy(ctx context.Context, opts *DeployOptions, args ...string) ([]kraftcloudinstances.Instance, error) {
	var pkgName string

	if len(opts.Name) > 0 {
		pkgName = opts.Name
	} else if opts.Project != nil && len(opts.Project.Name()) > 0 {
		pkgName = opts.Project.Name()
	} else {
		pkgName = filepath.Base(opts.Workdir)
	}

	if strings.HasPrefix(pkgName, "unikraft.io") {
		pkgName = "index." + pkgName
	}
	if !strings.HasPrefix(pkgName, "index.unikraft.io") {
		pkgName = fmt.Sprintf(
			"index.unikraft.io/%s/%s:latest",
			strings.TrimSuffix(strings.TrimPrefix(opts.Auth.User, "robot$"), ".users.kraftcloud"),
			pkgName,
		)
	}

	packs, err := pkg.Pkg(ctx, &pkg.PkgOptions{
		Architecture: "x86_64",
		Format:       "oci",
		Kraftfile:    opts.Kraftfile,
		Name:         pkgName,
		Platform:     "kraftcloud",
		Project:      opts.Project,
		Push:         true,
		Strategy:     opts.Strategy,
		Workdir:      opts.Workdir,
	})
	if err != nil {
		return nil, fmt.Errorf("could not package: %w", err)
	}

	// TODO(nderjung): This is a quirk that will be removed.  Remove the `index.`
	// from the name.
	if pkgName[0:17] == "index.unikraft.io" {
		pkgName = pkgName[6:]
	}
	if pkgName[0:12] == "unikraft.io/" {
		pkgName = pkgName[12:]
	}

	// FIXME(nderjung): Gathering the digest like this really dirty.
	metadata := packs[0].Columns()
	var digest string
	for _, m := range metadata {
		if m.Name != "index" {
			continue
		}

		digest = m.Value
	}

	var instance *kraftcloudinstances.Instance

	ctx, deployCancel := context.WithTimeout(ctx, 60*time.Second)
	defer deployCancel()

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
				var ctxTimeout context.Context
				var cancel context.CancelFunc

			checkRemoteImages:
				for {
					// First check if the context has been cancelled
					select {
					case <-ctx.Done():
						return fmt.Errorf("context cancelled")
					default:
					}

					// Introduce a new context that is used only for iteration
					ctxTimeout, cancel = context.WithTimeout(context.TODO(), 5*time.Second)
					defer cancel()

					images, err := opts.Client.Images().WithMetro(opts.Metro).List(ctxTimeout)
					if err != nil && strings.HasSuffix(err.Error(), "context deadline exceeded") {
						continue
					} else if err != nil {
						return fmt.Errorf("could not check list of images: %w", err)
					}

					for _, image := range images {
						split := strings.Split(image.Digest, "@sha256:")
						if !strings.HasPrefix(split[len(split)-1], digest) {
							continue
						}

						cancel()
						break checkRemoteImages
					}
				}

			attemptDeployment:
				for {
					select {
					case <-ctx.Done():
						return fmt.Errorf("context cancelled")
					default:
					}

					// Introduce a new context that is used only for iteration
					ctxTimeout, cancel = context.WithTimeout(context.TODO(), 5*time.Second)
					defer cancel()

					instance, err = create.Create(ctxTimeout, &create.CreateOptions{
						Env:       opts.Env,
						FQDN:      opts.FQDN,
						Image:     pkgName,
						Memory:    opts.Memory,
						Metro:     opts.Metro,
						Name:      opts.Name,
						Ports:     opts.Ports,
						Replicas:  opts.Replicas,
						Start:     !opts.NoStart,
						SubDomain: opts.SubDomain,
					}, args...)
					if err != nil && strings.HasSuffix(err.Error(), "context deadline exceeded") {
						continue
					} else if err != nil {
						return fmt.Errorf("could not create instance: %w", err)
					}

					cancel()
					break attemptDeployment
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

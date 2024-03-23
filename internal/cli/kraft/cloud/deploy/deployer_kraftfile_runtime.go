// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	"kraftkit.sh/internal/cli/kraft/pkg"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/processtree"
)

type deployerKraftfileRuntime struct {
	args []string
}

func (deployer *deployerKraftfileRuntime) Name() string {
	return "kraftfile-runtime"
}

func (deployer *deployerKraftfileRuntime) String() string {
	if len(deployer.args) == 0 {
		return "run the cwd with Kraftfile"
	}

	return fmt.Sprintf("run the cwd's Kraftfile and use '%s' as arg(s)", strings.Join(deployer.args, " "))
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

	deployer.args = args

	return true, nil
}

func (deployer *deployerKraftfileRuntime) Deploy(ctx context.Context, opts *DeployOptions, args ...string) ([]kcinstances.GetResponseItem, []kcservices.GetResponseItem, error) {
	var pkgName string

	if len(opts.Name) > 0 {
		pkgName = opts.Name
	} else if opts.Project != nil && len(opts.Project.Name()) > 0 {
		pkgName = opts.Project.Name()
	} else {
		pkgName = filepath.Base(opts.Workdir)
	}

	user := strings.TrimSuffix(strings.TrimPrefix(opts.Auth.User, "robot$"), ".users.kraftcloud")
	if split := strings.Split(pkgName, "/"); len(split) > 1 {
		user = split[0]
		pkgName = strings.Join(split[1:], "/")
	}

	if strings.HasPrefix(pkgName, "unikraft.io") {
		pkgName = "index." + pkgName
	}
	if !strings.HasPrefix(pkgName, "index.unikraft.io") {
		pkgName = fmt.Sprintf(
			"index.unikraft.io/%s/%s:latest",
			user,
			pkgName,
		)
	}

	packs, err := pkg.Pkg(ctx, &pkg.PkgOptions{
		Architecture: "x86_64",
		Compress:     opts.Compress,
		Format:       "oci",
		Kraftfile:    opts.Kraftfile,
		Name:         pkgName,
		NoPull:       true,
		Platform:     "kraftcloud",
		Project:      opts.Project,
		Push:         true,
		Rootfs:       opts.Rootfs,
		Strategy:     opts.Strategy,
		Workdir:      opts.Workdir,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not package: %w", err)
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

	var inst *kcinstances.GetResponseItem
	var sg *kcservices.GetResponseItem

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
			processtree.WithHideOnSuccess(false),
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

					imagesResp, err := opts.Client.Images().WithMetro(opts.Metro).List(ctxTimeout)
					if err != nil {
						if errors.Is(err, context.DeadlineExceeded) {
							continue
						}
						return fmt.Errorf("could not check list of images: %w", err)
					}
					images, err := imagesResp.AllOrErr()
					if err != nil {
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

					inst, sg, err = create.Create(ctxTimeout, &create.CreateOptions{
						Env:                    opts.Env,
						FQDN:                   opts.FQDN,
						Image:                  pkgName,
						Memory:                 opts.Memory,
						Metro:                  opts.Metro,
						Name:                   strings.ReplaceAll(opts.Name, "/", "-"),
						Ports:                  opts.Ports,
						Replicas:               opts.Replicas,
						ScaleToZero:            opts.ScaleToZero,
						ServiceGroupNameOrUUID: opts.ServiceGroupNameOrUUID,
						Start:                  !opts.NoStart,
						SubDomain:              opts.SubDomain,
						Token:                  opts.Token,
						Volumes:                opts.Volumes,
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
		return nil, nil, err
	}

	if err := paramodel.Start(); err != nil {
		return nil, nil, err
	}

	if sg == nil {
		return []kcinstances.GetResponseItem{*inst}, []kcservices.GetResponseItem{}, nil
	}

	return []kcinstances.GetResponseItem{*inst}, []kcservices.GetResponseItem{*sg}, nil
}

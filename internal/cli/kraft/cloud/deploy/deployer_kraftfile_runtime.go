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

	kcclient "sdk.kraft.cloud/client"
	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	"kraftkit.sh/internal/cli/kraft/pkg"
	"kraftkit.sh/log"
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

func (deployer *deployerKraftfileRuntime) Deploy(ctx context.Context, opts *DeployOptions, args ...string) (*kcclient.ServiceResponse[kcinstances.GetResponseItem], *kcclient.ServiceResponse[kcservices.GetResponseItem], error) {
	var pkgName string

	if opts.Image != "" {
		pkgName = opts.Image
	} else if opts.Name != "" {
		pkgName = opts.Name
	} else if opts.Project != nil && opts.Project.Name() != "" {
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
		Runtime:      opts.Runtime,
		Strategy:     opts.Strategy,
		Workdir:      opts.Workdir,
	})
	if err != nil {
		if strings.Contains(err.Error(), "DENIED") && strings.Contains(err.Error(), "exceed") {
			log.G(ctx).Warn("storage quota exceeded. please delete some images and try again in at most 15 minutes.")
			log.G(ctx).Warn("see: kraft cloud image ls --help")
			log.G(ctx).Warn("see: kraft cloud image delete --help")
		}
		return nil, nil, fmt.Errorf("could not package: %w", err)
	}

	return create.Create(ctx, &create.CreateOptions{
		Certificate:         opts.Certificate,
		Env:                 opts.Env,
		Domain:              opts.Domain,
		Image:               packs[0].ID(),
		Memory:              opts.Memory,
		Metro:               opts.Metro,
		Name:                opts.Name,
		Ports:               opts.Ports,
		Replicas:            opts.Replicas,
		RestartPolicy:       &opts.RestartPolicy,
		Rollout:             &opts.Rollout,
		RolloutQualifier:    &opts.RolloutQualifier,
		RolloutWait:         opts.RolloutWait,
		ScaleToZero:         opts.ScaleToZero,
		ScaleToZeroStateful: opts.ScaleToZeroStateful,
		ScaleToZeroCooldown: opts.ScaleToZeroCooldown,
		ServiceNameOrUUID:   opts.ServiceNameOrUUID,
		Start:               !opts.NoStart,
		SubDomain:           opts.SubDomain,
		Token:               opts.Token,
		Volumes:             opts.Volumes,
		WaitForImage:        true,
		WaitForImageTimeout: opts.Timeout,
	}, deployer.args...)
}

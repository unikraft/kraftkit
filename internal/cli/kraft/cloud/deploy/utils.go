// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"path/filepath"
	"strconv"
	"time"

	"kraftkit.sh/unikraft/app"
	kcinstances "sdk.kraft.cloud/instances"
)

const (
	LabelScaleToZeroPolicy   = "cloud.unikraft.v1.instances/scale_to_zero.policy"
	LabelScaleToZeroStateful = "cloud.unikraft.v1.instances/scale_to_zero.stateful"
	LabelScaleToZeroCooldown = "cloud.unikraft.v1.instances/scale_to_zero.cooldown_time_ms"
)

// initProject sets up the project based on the provided context and
// options.
func (opts *DeployOptions) initProject(ctx context.Context) error {
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

	for k, v := range opts.Project.Labels() {
		switch k {
		case LabelScaleToZeroPolicy:
			if opts.ScaleToZero == nil {
				policy := kcinstances.ScaleToZeroPolicy(v)
				opts.ScaleToZero = &policy
			}
		case LabelScaleToZeroStateful:
			if opts.ScaleToZeroStateful == nil {
				stateful, err := strconv.ParseBool(v)
				if err != nil {
					return err
				}
				opts.ScaleToZeroStateful = &stateful
			}
		case LabelScaleToZeroCooldown:
			if opts.ScaleToZeroCooldown == 0 {
				cooldown, err := strconv.ParseInt(v, 10, 32)
				if err != nil {
					return err
				}
				opts.ScaleToZeroCooldown = time.Duration(cooldown) * time.Millisecond
			}
		}
	}

	return nil
}

func updateOptsFromProject(opts *DeployOptions) {
	if opts.Project != nil && opts.Project.Rootfs() != "" && opts.Rootfs == "" {
		if filepath.IsAbs(opts.Project.Rootfs()) {
			opts.Rootfs = opts.Project.Rootfs()
		} else {
			opts.Rootfs = filepath.Join(opts.Workdir, opts.Project.Rootfs())
		}
	}

	if opts.Project != nil && opts.Project.InitrdFsType().String() != "" && opts.RootfsType == "" {
		opts.RootfsType = opts.Project.InitrdFsType()
	}
}

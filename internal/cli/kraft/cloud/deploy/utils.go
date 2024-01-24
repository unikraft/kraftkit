// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"fmt"

	"kraftkit.sh/config"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft/app"
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

	return nil
}

// checkExists determines if the provided deployment (with the same name)
// already exists and, if possible, prompts the user for the rollout strategy.
func (opts *DeployOptions) checkExists(ctx context.Context) error {
	// Check if the instance with the same name already exists
	exists, err := opts.Client.Instances().GetByName(ctx, opts.Name)
	if err == nil && exists != nil {
		if opts.Rollout == StrategyPrompt {
			if config.G[config.KraftKit](ctx).NoPrompt {
				return fmt.Errorf("prompting disabled")
			}

			strategy, err := selection.Select[RolloutStrategy](
				fmt.Sprintf("deployment with name '%s' already exists: how would you like to proceed?", opts.Name),
				RolloutStrategies()...,
			)
			if err != nil {
				return err
			}

			opts.Rollout = *strategy
		}

		switch opts.Rollout {
		case StrategyExit:
			return fmt.Errorf("deployment already exists and merge strategy set to exit on conflict")

		case StrategyOverwrite:
			if err := opts.Client.Instances().DeleteByName(ctx, opts.Name); err != nil {
				return fmt.Errorf("could not delete deployment '%s': %w", opts.Name, err)
			}

		default:
			return fmt.Errorf("unsupported rollout strategy '%s'", opts.Rollout.String())
		}
	}

	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package add

import (
	"context"
	"fmt"
	"sort"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	cloud "sdk.kraft.cloud"
	ukcautoscale "sdk.kraft.cloud/services/autoscale"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type AddOptions struct {
	Adjustment string             `long:"adjustment" short:"a" usage:"The adjustment of the policy. Valid options: 'percent', 'absolute', 'change'" default:"change"`
	Auth       *config.AuthConfig `noattribute:"true"`
	Client     cloud.KraftCloud   `noattribute:"true"`
	Metric     string             `long:"metric" short:"m" usage:"The metric of the policy. Valid options: 'cpu'" default:"cpu"`
	Metro      string             `noattribute:"true"`
	Name       string             `long:"name" short:"n" usage:"The name of the policy"`
	Step       []string           `long:"step" short:"s" usage:"The step of the policy in the format 'LOWER_BOUND:UPPER_BOUND/ADJUSTMENT'"`
	Token      string             `noattribute:"true"`
}

// stepFormat holds the step format of the policy for parsing.
type stepFormat struct {
	LowerBound int
	UpperBound int
	Adjustment int
	LowerEmpty bool
	UpperEmpty bool
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&AddOptions{}, cobra.Command{
		Short:   "Add an autoscale configuration policy",
		Use:     "add [FLAGS] UUID|NAME",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"a"},
		Long:    "Add an autoscale configuration policy for a service.",
		Example: heredoc.Doc(`
			# Add an autoscale configuration policy by service UUID
			$ kraft cloud scale add fd1684ea-7970-4994-92d6-61dcc7905f2b --name my-policy --step 0:10/1

			# Add an autoscale configuration policy by service name
			$ kraft cloud scale add my-service --name my-policy --step 0:10/1 --step 10:20/2
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-scale",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *AddOptions) Pre(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("specify a configuration UUID or NAME")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *AddOptions) Run(ctx context.Context, args []string) error {
	var err error

	if opts.Name == "" {
		return fmt.Errorf("specify a policy name")
	}

	if len(opts.Step) < 1 || len(opts.Step) > 4 {
		return fmt.Errorf("specify between 1 and 4 steps")
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetUnikraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = cloud.NewClient(
			cloud.WithToken(config.GetUnikraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	id := args[0]

	// Look up the configuration by name
	if !utils.IsUUID(id) {
		confResp, err := opts.Client.Services().WithMetro(opts.Metro).Get(ctx, id)
		if err != nil {
			return fmt.Errorf("could not get configuration: %w", err)
		}
		conf, err := confResp.FirstOrErr()
		if err != nil {
			return fmt.Errorf("could not get configuration: %w", err)
		}

		id = conf.UUID
	}

	steps := []stepFormat{}
	for _, step := range opts.Step {
		var policyStep stepFormat

		// Read in all possible format. Fail otherwise.
		if _, err := fmt.Sscanf(step, "%d:%d/%d", &policyStep.LowerBound, &policyStep.UpperBound, &policyStep.Adjustment); err != nil {
			if _, err := fmt.Sscanf(step, ":%d/%d", &policyStep.UpperBound, &policyStep.Adjustment); err != nil {
				if _, err := fmt.Sscanf(step, "%d:/%d", &policyStep.LowerBound, &policyStep.Adjustment); err != nil {
					return fmt.Errorf("could not parse step '%s': expected format 'LOWER_BOUND:UPPER_BOUND/ADJUSTMENT'", step)
				} else {
					policyStep.UpperEmpty = true
				}
			} else {
				policyStep.LowerEmpty = true
			}
		}

		if policyStep.LowerBound >= policyStep.UpperBound && !policyStep.LowerEmpty && !policyStep.UpperEmpty {
			return fmt.Errorf("lower bound cannot be greater or equal than upper bound")
		}

		steps = append(steps, policyStep)
	}

	// Sort steps by lower bound.
	sort.Slice(steps, func(i, j int) bool {
		// First one should be the empty one
		if steps[i].LowerEmpty && !steps[j].LowerEmpty {
			return true
		}
		return steps[i].LowerBound < steps[j].LowerBound
	})

	// Do checks to ensure the steps are contiguous and valid.
	for idx, step := range steps {
		if idx == 0 {
			continue
		}

		if step.LowerEmpty {
			return fmt.Errorf("lower bound cannot be empty in a step after the first step")
		}

		if step.UpperEmpty && idx != len(opts.Step)-1 {
			return fmt.Errorf("upper bound cannot be empty in a step before the last step")
		}

		if steps[idx-1].UpperBound != step.LowerBound {
			return fmt.Errorf("steps are not contiguous, gap found between %d and %d", steps[idx-1].UpperBound, step.LowerBound)
		}
	}

	stepPol := ukcautoscale.StepPolicy{
		Name:           opts.Name,
		Metric:         ukcautoscale.PolicyMetric(opts.Metric),
		AdjustmentType: ukcautoscale.AdjustmentType(opts.Adjustment),
	}
	for _, step := range steps {
		s := ukcautoscale.Step{
			Adjustment: step.Adjustment,
		}
		if !step.LowerEmpty {
			s.LowerBound = &step.LowerBound
		}
		if !step.UpperEmpty {
			s.UpperBound = &step.UpperBound
		}
		stepPol.Steps = append(stepPol.Steps, s)
	}

	addPolicyResp, err := opts.Client.Autoscale().WithMetro(opts.Metro).AddPolicy(ctx, id, stepPol)
	if err != nil {
		return fmt.Errorf("could not add configuration: %w", err)
	}
	if _, err = addPolicyResp.AllOrErr(); err != nil {
		return fmt.Errorf("could not add configuration: %w", err)
	}

	return nil
}

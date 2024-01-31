// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package add

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kraftcloudautoscale "sdk.kraft.cloud/services/autoscale"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type AddOptions struct {
	Adjustment string                `long:"adjustment" short:"a" usage:"The adjustment of the policy. Valid options: 'percent', 'absolute', 'change'" default:"change"`
	Auth       *config.AuthConfig    `noattribute:"true"`
	Client     kraftcloud.KraftCloud `noattribute:"true"`
	Metric     string                `long:"metric" short:"m" usage:"The metric of the policy. Valid options: 'cpu'" default:"cpu"`
	Metro      string                `noattribute:"true"`
	Name       string                `long:"name" short:"n" usage:"The name of the policy"`
	Step       []string              `long:"step" short:"s" usage:"The step of the policy in the format 'LOWER_BOUND:UPPER_BOUND/ADJUSTMENT'"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&AddOptions{}, cobra.Command{
		Short:   "Add an autoscale configuration policy",
		Use:     "add [FLAGS] UUID|NAME",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"a"},
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-scale",
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

	opts.Metro = cmd.Flag("metro").Value.String()
	if opts.Metro == "" {
		opts.Metro = os.Getenv("KRAFTCLOUD_METRO")
	}
	if opts.Metro == "" {
		return fmt.Errorf("kraftcloud metro is unset")
	}

	log.G(cmd.Context()).WithField("metro", opts.Metro).Debug("using")

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
		opts.Auth, err = config.GetKraftCloudAuthConfigFromContext(ctx)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	// Look up the configuration by name
	if !utils.IsUUID(args[0]) {
		conf, err := opts.Client.Services().WithMetro(opts.Metro).GetByName(ctx, args[0])
		if err != nil {
			return fmt.Errorf("could not get configuration: %w", err)
		}

		args[0] = conf.UUID
	}

	steps := []kraftcloudautoscale.AutoscaleStepPolicyStep{}

	for _, step := range opts.Step {
		var policyStep kraftcloudautoscale.AutoscaleStepPolicyStep

		if _, err := fmt.Sscanf(step, "%d:%d/%d", &policyStep.LowerBound, &policyStep.UpperBound, &policyStep.Adjustment); err != nil {
			return fmt.Errorf("could not parse step '%s': expected format 'LOWER_BOUND:UPPER_BOUND/ADJUSTMENT'", step)
		}

		// if lowerBoundString == "" && upperBoundString == "" {
		// 	return fmt.Errorf("lower bound and upper bound cannot both be null in the same step")
		// }

		// if idx != 0 && lowerBoundString == "" {
		// 	return fmt.Errorf("lower bound cannot be null in a step after the first step")
		// }

		// if idx != len(opts.Step)-1 && upperBoundString == "" {
		// 	return fmt.Errorf("upper bound cannot be null in a step before the last step")
		// }

		if policyStep.Adjustment == 0 {
			return fmt.Errorf("adjustment cannot be zero")
		}

		// if lowerBoundString != "" {
		// 	lowerBound, err = strconv.Atoi(lowerBoundString)
		// 	if err != nil {
		// 		return fmt.Errorf("could not parse lower bound: %w", err)
		// 	}
		// }

		// if upperBoundString != "" {
		// 	upperBound, err = strconv.Atoi(upperBoundString)
		// 	if err != nil {
		// 		return fmt.Errorf("could not parse upper bound: %w", err)
		// 	}
		// }

		// adjustment, err = strconv.Atoi(adjustmentString)
		// if err != nil {
		// 	return fmt.Errorf("could not parse adjustment: %w", err)
		// }

		if policyStep.LowerBound >= policyStep.UpperBound {
			return fmt.Errorf("lower bound cannot be greater or equal than upper bound")
		}

		// if adjustment == 0 {
		// 	return fmt.Errorf("adjustment cannot be 0")
		// }

		// if lowerBoundString != "" {
		// 	policyStep.LowerBound = lowerBound
		// }
		// if upperBoundString != "" {
		// 	policyStep.UpperBound = upperBound
		// }

		// policyStep.Adjustment = adjustment

		steps = append(steps, policyStep)
	}

	// Sort steps by lower bound.
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].LowerBound < steps[j].LowerBound
	})

	for idx, step := range steps {
		if idx == 0 {
			continue
		}

		if steps[idx-1].UpperBound != step.LowerBound {
			return fmt.Errorf("steps are not contiguous, gap found between %d and %d", steps[idx-1].UpperBound, step.LowerBound)
		}
	}

	if _, err := opts.Client.
		Autoscale().
		WithMetro(opts.Metro).
		CreatePolicy(ctx,
			args[0],
			kraftcloudautoscale.AutoscalePolicyTypeStep,
			&kraftcloudautoscale.AutoscaleStepPolicy{
				Name:           opts.Name,
				AdjustmentType: kraftcloudautoscale.AutoscaleAdjustmentType(opts.Adjustment),
				Metric:         kraftcloudautoscale.AutoscalePolicyMetric(opts.Metric),
				Steps:          steps,
			},
		); err != nil {
		return fmt.Errorf("could not add configuration: %w", err)
	}

	return nil
}

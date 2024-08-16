// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package metrics

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type MetricsOptions struct {
	Auth   *config.AuthConfig    `noattribute:"true"`
	Client kraftcloud.KraftCloud `noattribute:"true"`
	Metro  string                `noattribute:"true"`
	Token  string                `noattribute:"true"`
	Output string                `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&MetricsOptions{}, cobra.Command{
		Short:   "Return metrics for instances",
		Use:     "top [FLAGS] [UUID|NAME [UUID|NAME]...]",
		Aliases: []string{"metrics", "metric", "m", "meter"},
		Args:    cobra.ArbitraryArgs,
		Example: heredoc.Doc(`
			# Return metrics for all instances
			$ kraft cloud instance top

			# Return metrics for an instance by UUID
			$ kraft cloud instance top fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Return metrics for an instance by name
			$ kraft cloud instance top my-instance-431342
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-instance",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *MetricsOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	return nil
}

func (opts *MetricsOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.Token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewInstancesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	instances := args
	if len(instances) == 0 {
		resp, err := client.WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}

		insts, err := resp.AllOrErr()
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}

		for _, inst := range insts {
			instances = append(instances, inst.UUID)
		}
	}

	if len(instances) > 0 {
		if opts.Output == "" && len(instances) > 1 {
			opts.Output = "table"
		} else if opts.Output == "" && len(instances) == 1 {
			opts.Output = "list"
		}

		resp, err := client.WithMetro(opts.Metro).Metrics(ctx, instances...)
		if err != nil {
			return fmt.Errorf("could not get instance %s: %w", instances, err)
		}
		return utils.PrintMetrics(ctx, opts.Output, *resp)
	}

	return fmt.Errorf("no instances found")
}

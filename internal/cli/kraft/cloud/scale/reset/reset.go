// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package reset

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	cloud "sdk.kraft.cloud"
	ukcautoscale "sdk.kraft.cloud/services/autoscale"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type ResetOptions struct {
	Auth   *config.AuthConfig            `noattribute:"true"`
	Client ukcautoscale.AutoscaleService `noattribute:"true"`
	Metro  string                        `noattribute:"true"`
	Token  string                        `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ResetOptions{}, cobra.Command{
		Short:   "Reset autoscale configuration of a service",
		Use:     "reset [FLAGS] UUID|NAME",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"rs", "delconfig", "deinit", "rmconfig"},
		Example: heredoc.Doc(`
			# Reset an autoscale configuration by UUID
			$ kraft cloud scale reset fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Reset an autoscale configuration by name
			$ kraft cloud scale reset my-service
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

func (opts *ResetOptions) Pre(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("specify a service name or UUID")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *ResetOptions) Run(ctx context.Context, args []string) error {
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetUnikraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = cloud.NewAutoscaleClient(
			cloud.WithToken(config.GetUnikraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	delResp, err := opts.Client.WithMetro(opts.Metro).DeleteConfigurations(ctx, args[0])
	if err != nil {
		return fmt.Errorf("could not reset configuration: %w", err)
	}
	if _, err := delResp.AllOrErr(); err != nil {
		return fmt.Errorf("could not reset configuration: %w", err)
	}

	return nil
}

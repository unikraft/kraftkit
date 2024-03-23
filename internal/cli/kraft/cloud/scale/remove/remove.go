// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcautoscale "sdk.kraft.cloud/services/autoscale"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type RemoveOptions struct {
	Auth   *config.AuthConfig           `noattribute:"true"`
	Client kcautoscale.AutoscaleService `noattribute:"true"`
	Metro  string                       `noattribute:"true"`
	Token  string                       `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Delete an autoscale configuration policy",
		Use:     "remove [FLAGS] UUID NAME",
		Args:    cobra.ExactArgs(2),
		Aliases: []string{"delete", "del", "rm"},
		Long:    "Delete an autoscale configuration policy.",
		Example: heredoc.Doc(`
			# Delete an autoscale configuration policy by UUID
			$ kraft cloud scale remove fd1684ea-7970-4994-92d6-61dcc7905f2b my-policy

			# Delete an autoscale configuration policy by name
			$ kraft cloud scale remove my-service-group my-policy
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-scale",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *RemoveOptions) Pre(cmd *cobra.Command, args []string) error {
	if len(args) == 0 || len(args) == 1 {
		return fmt.Errorf("specify service group UUID and policy name")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	var err error

	if !utils.IsUUID(args[0]) {
		return fmt.Errorf("specify a valid service group UUID")
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewAutoscaleClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	_, err = opts.Client.WithMetro(opts.Metro).DeletePolicyByName(ctx, args[0], args[1])
	if err != nil {
		return fmt.Errorf("could not delete policy: %w", err)
	}

	return nil
}

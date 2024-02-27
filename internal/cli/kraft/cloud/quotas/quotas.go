// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package quotas

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"

	kraftcloud "sdk.kraft.cloud"
)

type QuotasOptions struct {
	Limits bool   `long:"limits" short:"l" usage:"Show usage limits"`
	Output string `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`

	metro string
	token string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&QuotasOptions{}, cobra.Command{
		Short:   "View your resource quota on KraftCloud",
		Use:     "quotas",
		Args:    cobra.NoArgs,
		Aliases: []string{"q", "quota"},
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud",
		},
		Long: heredoc.Doc(`
			View your resource quota on KraftCloud.
		`),
		Example: heredoc.Doc(`
			# View your resource quota on KraftCloud
			$ kraft cloud quota

			# View your resource quota on KraftCloud in JSON format
			$ kraft cloud quota -o json
		`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *QuotasOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *QuotasOptions) Run(ctx context.Context, _ []string) error {
	auth, err := config.GetKraftCloudAuthConfigFromContext(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewUsersClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	quotas, err := client.WithMetro(opts.metro).Quotas(ctx)
	if err != nil {
		return fmt.Errorf("could not get quotas: %w", err)
	}

	if opts.Limits {
		return utils.PrintLimits(ctx, opts.Output, *quotas)
	} else {
		return utils.PrintQuotas(ctx, opts.Output, *quotas)
	}
}

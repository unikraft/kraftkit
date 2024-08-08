// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package quota

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"

	kraftcloud "sdk.kraft.cloud"
)

type QuotaOptions struct {
	Output string `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"list"`

	metro string
	token string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&QuotaOptions{}, cobra.Command{
		Short:   "View your resource quota on Unikraft Cloud",
		Use:     "quota",
		Args:    cobra.NoArgs,
		Aliases: []string{"q", "quotas"},
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud",
		},
		Example: heredoc.Doc(`
			# View your resource quota on Unikraft Cloud
			$ kraft cloud quota

			# View your resource quota on Unikraft Cloud in JSON format
			$ kraft cloud quota -o json
		`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *QuotaOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	return nil
}

func (opts *QuotaOptions) Run(ctx context.Context, _ []string) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	resp, err := client.Users().WithMetro(opts.metro).Quotas(ctx)
	if err != nil {
		return fmt.Errorf("could not get quotas: %w", err)
	}

	imageResp, err := client.Images().WithMetro(opts.metro).Quotas(ctx)
	if err != nil {
		return fmt.Errorf("could not get image quotas: %w", err)
	}

	if err = iostreams.G(ctx).StartPager(); err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	}

	defer iostreams.G(ctx).StopPager()

	return utils.PrintQuotas(ctx, *auth, opts.Output, *resp, imageResp)
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package list

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

type ListOptions struct {
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list,raw" default:"table"`

	metro string
	token string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ListOptions{}, cobra.Command{
		Short:   "List instance templates",
		Use:     "list",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Long: heredoc.Doc(`
			List all instance templates in your account.
		`),
		Example: heredoc.Doc(`
			# List all instance templates in your account.
			$ kraft cloud instance template list

			# List all instance templates in your account in JSON format.
			$ kraft cloud instance template list -o json
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-instance-template",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *ListOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	return nil
}

func (opts *ListOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewInstancesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	resp, err := client.WithMetro(opts.metro).ListTemplate(ctx)
	if err != nil {
		return fmt.Errorf("could not list instance: %w", err)
	}

	return utils.PrintInstancesTemplates(ctx, opts.Output, *resp)
}

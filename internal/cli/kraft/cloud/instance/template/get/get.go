// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package get

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

type GetOptions struct {
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list,raw" default:"list"`

	metro         string
	token         string
	allowInsecure bool
}

// Status of a Unikraft Cloud instance template.
func Status(ctx context.Context, opts *GetOptions, args ...string) error {
	if opts == nil {
		opts = &GetOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&GetOptions{}, cobra.Command{
		Short:   "Retrieve the state of instance templates",
		Use:     "get [FLAGS] UUID|NAME",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"gt"},
		Example: heredoc.Doc(`
			# Retrieve information about a Unikraft Cloud instance template by UUID
			$ kraft cloud instance template get fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Retrieve information about two Unikraft Cloud templates by name
			$ kraft cloud instance get my-instance-template-1 my-instance-template-2
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

func (opts *GetOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token, &opts.allowInsecure)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	return nil
}

func (opts *GetOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewInstancesClient(
		kraftcloud.WithAllowInsecure(opts.allowInsecure),
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	resp, err := client.WithMetro(opts.metro).GetTemplate(ctx, args...)
	if err != nil {
		return fmt.Errorf("could not get instance %v: %w", args, err)
	}

	return utils.PrintInstancesTemplates(ctx, opts.Output, *resp)
}

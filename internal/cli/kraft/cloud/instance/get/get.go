// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package get

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	cloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type GetOptions struct {
	Auth   *config.AuthConfig `noattribute:"true"`
	Client cloud.KraftCloud   `noattribute:"true"`
	Metro  string             `noattribute:"true"`
	Token  string             `noattribute:"true"`
	Output string             `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"list"`
}

// Status of a UnikraftCloud instance.
func Get(ctx context.Context, opts *GetOptions, args ...string) error {
	if opts == nil {
		opts = &GetOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&GetOptions{}, cobra.Command{
		Short:   "Retrieve the state of instances",
		Use:     "get [FLAGS] [UUID|NAME]",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"status", "info"},
		Example: heredoc.Doc(`
			# Retrieve information about a instance by UUID
			$ kraft cloud instance get fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Retrieve information about a instance by name
			$ kraft cloud instance get my-instance-431342
		`),
		Long: heredoc.Doc(`
			Retrieve the state of an instance.
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-instance",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *GetOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	return nil
}

func (opts *GetOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetUnikraftCloudAuthConfig(ctx, opts.Token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := cloud.NewInstancesClient(
		cloud.WithToken(config.GetUnikraftCloudTokenAuthConfig(*auth)),
	)

	resp, err := client.WithMetro(opts.Metro).Get(ctx, args...)
	if err != nil {
		return fmt.Errorf("could not get instance %s: %w", args, err)
	}

	return utils.PrintInstances(ctx, opts.Output, *resp)
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
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
	"kraftkit.sh/log"
)

type ListOptions struct {
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`

	metro string
	token string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ListOptions{}, cobra.Command{
		Short:   "List instances",
		Use:     "list [FLAGS]",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			# List all instances in your account.
			$ kraft cloud instance list
		`),
		Long: heredoc.Doc(`
			List all instances in your account.
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

func (opts *ListOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.metro = cmd.Flag("metro").Value.String()
	if opts.metro == "" {
		return fmt.Errorf("kraftcloud metro is unset")
	}
	log.G(cmd.Context()).WithField("metro", opts.metro).Debug("using")

	opts.token = cmd.Flag("token").Value.String()
	if opts.token == "" {
		return fmt.Errorf("kraftcloud token is unset")
	}

	log.G(cmd.Context()).WithField("token", opts.token).Debug("using")

	return nil
}

func (opts *ListOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfigFromContext(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewInstancesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	instances, err := client.WithMetro(opts.metro).List(ctx)
	if err != nil {
		return fmt.Errorf("could not list instances: %w", err)
	}

	return utils.PrintInstances(ctx, opts.Output, instances...)
}

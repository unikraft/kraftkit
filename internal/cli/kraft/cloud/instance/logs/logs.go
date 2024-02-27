// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package logs

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/iostreams"
)

type LogOptions struct {
	Tail int `local:"true" long:"tail" short:"n" usage:"Lines of recent logs to display" default:"-1"`

	metro string
	token string
}

// Log retrieves the console output from a KraftCloud instance.
func Log(ctx context.Context, opts *LogOptions, args ...string) error {
	if opts == nil {
		opts = &LogOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&LogOptions{}, cobra.Command{
		Short:   "Get console output of an instance",
		Use:     "logs [FLAG] UUID|NAME",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"log"},
		Example: heredoc.Doc(`
			# Get console output of a kraftcloud instance by UUID
			$ kraft cloud instance logs 77d0316a-fbbe-488d-8618-5bf7a612477a

			# Get console output of a kraftcloud instance by name
			$ kraft cloud instance logs my-instance-431342
		`),
		Long: heredoc.Doc(`
			Get console output of an instance.
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

func (opts *LogOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *LogOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfigFromContext(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewInstancesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	var logs string

	if utils.IsUUID(args[0]) {
		logs, err = client.WithMetro(opts.metro).LogsByUUID(ctx, args[0], opts.Tail, true)
	} else {
		logs, err = client.WithMetro(opts.metro).LogsByName(ctx, args[0], opts.Tail, true)
	}
	if err != nil {
		return fmt.Errorf("could not retrieve logs: %w", err)
	}

	fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", logs)

	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package start

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

type StartOptions struct {
	WaitTimeoutMS int    `local:"true" long:"wait_timeout_ms" short:"w" usage:"Timeout to wait for the instance to start in milliseconds"`
	Output        string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`

	metro string
	token string
}

// Start a KraftCloud instance.
func Start(ctx context.Context, opts *StartOptions, args ...string) error {
	if opts == nil {
		opts = &StartOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StartOptions{}, cobra.Command{
		Short:   "Start an instance",
		Use:     "start [FLAGS] PACKAGE|NAME",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"str"},
		Example: heredoc.Doc(`
			# Start a KraftCloud instance by UUID
			$ kraft cloud instance start 77d0316a-fbbe-488d-8618-5bf7a612477a

			# Start a KraftCloud instance by name
			$ kraft cloud instance start my-instance-431342
		`),
		Long: heredoc.Doc(`
			Start an instance on KraftCloud from a stopped instance.
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

func (opts *StartOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *StartOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfig(ctx, opts.token)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewInstancesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	for _, arg := range args {
		log.G(ctx).Infof("starting %s", arg)

		if utils.IsUUID(arg) {
			_, err = client.WithMetro(opts.metro).StartByUUID(ctx, arg, opts.WaitTimeoutMS)
		} else {
			_, err = client.WithMetro(opts.metro).StartByName(ctx, arg, opts.WaitTimeoutMS)
		}
		if err != nil {
			log.G(ctx).WithError(err).Error("could not start instance")
			continue
		}
	}

	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
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
	"kraftkit.sh/internal/cli/kraft/cloud/instance/logs"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type LogOptions struct {
	AllowInsecure bool                  `noattribute:"true"`
	Auth          *config.AuthConfig    `noattribute:"true"`
	Client        kraftcloud.KraftCloud `noattribute:"true"`
	Follow        bool                  `local:"true" long:"follow" short:"f" usage:"Follow the logs of the service every half second" default:"false"`
	Metro         string                `noattribute:"true"`
	NoPrefix      bool                  `long:"no-prefix" usage:"When logging multiple machines, do not prefix each log line with the name"`
	Tail          int                   `local:"true" long:"tail" short:"n" usage:"Show the last given lines from the logs" default:"-1"`
	Token         string                `noattribute:"true"`
}

// Log retrieves the console output from a Unikraft Cloud service.
func Log(ctx context.Context, opts *LogOptions, args ...string) error {
	if opts == nil {
		opts = &LogOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&LogOptions{}, cobra.Command{
		Short:   "Get console output for services",
		Use:     "logs [FLAG] UUID|NAME",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"log"},
		Example: heredoc.Doc(`
			# Get all console output of a service by UUID
			$ kraft cloud service logs 77d0316a-fbbe-488d-8618-5bf7a612477a

			# Get all console output of a service by name
			$ kraft cloud service logs my-service-431342

			# Get the last 20 lines of a service by name
			$ kraft cloud service logs my-service-431342 --tail 20

			# Get the last lines of a service by name continuously
			$ kraft cloud service logs my-service-431342 --follow

			# Get the last 10 lines of a service by name continuously
			$ kraft cloud service logs my-service-431342 --follow --tail 10
		`),
		Long: heredoc.Doc(`
			Get console output of an service.
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-service",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *LogOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token, &opts.AllowInsecure)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if opts.Tail < -1 {
		return fmt.Errorf("invalid value for --tail: %d, should be -1 for all logs, or positive for length of truncated logs", opts.Tail)
	}

	return nil
}

func (opts *LogOptions) Run(ctx context.Context, args []string) error {
	return Logs(ctx, opts, args...)
}

func Logs(ctx context.Context, opts *LogOptions, args ...string) error {
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewClient(
			kraftcloud.WithAllowInsecure(opts.AllowInsecure),
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	var instances []string

	for _, service := range args {
		resp, err := opts.Client.Services().WithMetro(opts.Metro).Get(ctx, service)
		if err != nil {
			return err
		}

		item, err := resp.FirstOrErr()
		if err != nil {
			return err
		}

		for _, instance := range item.Instances {
			instances = append(instances, instance.UUID)
		}
	}

	return logs.Logs(ctx, &logs.LogOptions{
		Auth:     opts.Auth,
		Client:   opts.Client,
		Follow:   opts.Follow,
		Metro:    opts.Metro,
		NoPrefix: opts.NoPrefix,
		Tail:     opts.Tail,
		Token:    opts.Token,
	}, instances...)
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package start

import (
	"context"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type StartOptions struct {
	Wait   time.Duration `local:"true" long:"wait" short:"w" usage:"Timeout to wait for the instance to start (ms/s/m/h)"`
	Output string        `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`

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
		Use:     "start [FLAGS] [UUID|NAME [UUID|NAME]...]",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"str"},
		Example: heredoc.Doc(`
			# Start a KraftCloud instance by UUID
			$ kraft cloud instance start 77d0316a-fbbe-488d-8618-5bf7a612477a

			# Start a KraftCloud instance by name
			$ kraft cloud instance start my-instance-431342

			# Start multiple KraftCloud instances
			$ kraft cloud instance start my-instance-431342 my-instance-other-2313
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

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
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

	if opts.Wait < time.Millisecond && opts.Wait != 0 {
		return fmt.Errorf("wait timeout must be greater than 1ms")
	}

	timeout := int(opts.Wait.Milliseconds())
	for _, arg := range args {
		log.G(ctx).Infof("Starting %s", arg)

		if utils.IsUUID(arg) {
			_, err = client.WithMetro(opts.metro).StartByUUIDs(ctx, timeout, arg)
		} else {
			_, err = client.WithMetro(opts.metro).StartByNames(ctx, timeout, arg)
		}
		if err != nil {
			log.G(ctx).WithError(err).Error("could not start instance")
			continue
		}
	}

	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package stop

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type StopOptions struct {
	TimeoutMS int64  `local:"true" long:"timeout-ms" short:"w" usage:"Timeout for the instance to stop"`
	Output    string `long:"output" short:"o" usage:"Set output format" default:"table"`
	All       bool   `long:"all" usage:"Stop all instances"`
	Metro     string `noattribute:"true"`
}

// Stop a KraftCloud instance.
func Stop(ctx context.Context, opts *StopOptions, args ...string) error {
	if opts == nil {
		opts = &StopOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StopOptions{}, cobra.Command{
		Short: "Stop an instance",
		Use:   "stop [FLAGS] [UUID|NAME]",
		Args:  cobra.ArbitraryArgs,
		Example: heredoc.Doc(`
			# Stop a KraftCloud instance
			$ kraft cloud instance stop 77d0316a-fbbe-488d-8618-5bf7a612477a
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

func (opts *StopOptions) Pre(cmd *cobra.Command, args []string) error {
	if !opts.All && len(args) == 0 {
		return fmt.Errorf("either specify an instance UUID or --all flag")
	}

	opts.Metro = cmd.Flag("metro").Value.String()
	if opts.Metro == "" {
		opts.Metro = os.Getenv("KRAFTCLOUD_METRO")
	}
	if opts.Metro == "" {
		return fmt.Errorf("kraftcloud metro is unset")
	}
	log.G(cmd.Context()).WithField("metro", opts.Metro).Debug("using")
	return nil
}

func (opts *StopOptions) Run(ctx context.Context, args []string) error {
	auth, err := config.GetKraftCloudAuthConfigFromContext(ctx)
	if err != nil {
		return fmt.Errorf("could not retrieve credentials: %w", err)
	}

	client := kraftcloud.NewInstancesClient(
		kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*auth)),
	)

	if opts.All {
		instances, err := client.WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not get list of all instances: %w", err)
		}

		for _, instance := range instances {
			log.G(ctx).Infof("stopping %s", instance.UUID)

			_, err := client.WithMetro(opts.Metro).StopByUUID(ctx, instance.UUID, opts.TimeoutMS)
			if err != nil {
				log.G(ctx).Error("could not stop instance: %w", err)
			}
		}

		return nil
	}

	for _, arg := range args {
		log.G(ctx).Infof("stopping %s", arg)

		if utils.IsUUID(arg) {
			_, err = client.WithMetro(opts.Metro).StopByUUID(ctx, arg, opts.TimeoutMS)
		} else {
			_, err = client.WithMetro(opts.Metro).StopByName(ctx, arg, opts.TimeoutMS)
		}
		if err != nil {
			return fmt.Errorf("could not create instance: %w", err)
		}
	}

	return nil
}

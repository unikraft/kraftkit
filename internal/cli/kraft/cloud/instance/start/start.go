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
	All    bool                  `long:"all" short:"a" usage:"Start all instances"`
	Auth   *config.AuthConfig    `noattribute:"true"`
	Client kraftcloud.KraftCloud `noattribute:"true"`
	Metro  string                `noattribute:"true"`
	Token  string                `noattribute:"true"`
	Wait   time.Duration         `local:"true" long:"wait" short:"w" usage:"Timeout to wait for the instance to start (ms/s/m/h)"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StartOptions{}, cobra.Command{
		Short:   "Start instances",
		Use:     "start [FLAGS] [UUID|NAME [UUID|NAME]...]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"str"},
		Example: heredoc.Doc(`
			# Start an instance by UUID
			$ kraft cloud instance start 77d0316a-fbbe-488d-8618-5bf7a612477a

			# Start an instance by name
			$ kraft cloud instance start my-instance-431342

			# Start multiple instances
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
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate Metro and Token: %w", err)
	}

	return nil
}

func (opts *StartOptions) Run(ctx context.Context, args []string) error {
	return Start(ctx, opts, args...)
}

// Start KraftCloud instance(s).
func Start(ctx context.Context, opts *StartOptions, args ...string) error {
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	if opts.Wait < time.Millisecond && opts.Wait != 0 {
		return fmt.Errorf("wait timeout must be greater than 1ms")
	}

	timeout := int(opts.Wait.Milliseconds())

	if opts.All {
		args = []string{}

		instListResp, err := opts.Client.Instances().WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}

		instList, err := instListResp.AllOrErr()
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}
		if len(instList) == 0 {
			log.G(ctx).Info("no instances found")
			return nil
		}

		for _, instItem := range instList {
			args = append(args, instItem.UUID)
		}
	}

	log.G(ctx).Infof("starting %d instance(s)", len(args))

	resp, err := opts.Client.Instances().WithMetro(opts.Metro).Start(ctx, timeout, args...)
	if err != nil {
		return fmt.Errorf("starting instance: %w", err)
	}
	if _, err = resp.FirstOrErr(); err != nil {
		return fmt.Errorf("starting instance: %w", err)
	}

	return nil
}

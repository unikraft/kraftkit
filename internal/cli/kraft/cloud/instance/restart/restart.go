// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package restart

import (
	"context"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/start"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/stop"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type RestartOptions struct {
	AllowInsecure bool                  `noattribute:"true"`
	All           bool                  `long:"all" short:"a" usage:"restart all instances"`
	Auth          *config.AuthConfig    `noattribute:"true"`
	Client        kraftcloud.KraftCloud `noattribute:"true"`
	Force         bool                  `long:"force" short:"f" usage:"Force stop the instance(s)"`
	Metro         string                `noattribute:"true"`
	Token         string                `noattribute:"true"`
	Wait          time.Duration         `local:"true" long:"wait" short:"w" usage:"Timeout to wait for the instance to stop and/or start (ms/s/m/h)"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RestartOptions{}, cobra.Command{
		Short:   "Restart instance(s)",
		Use:     "restart [FLAGS] [UUID|NAME [UUID|NAME]...]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"rr"},
		Example: heredoc.Doc(`
			# Restart an instance by UUID
			$ kraft cloud instance restart 77d0316a-fbbe-488d-8618-5bf7a612477a

			# Restart an instance by name
			$ kraft cloud instance restart my-instance-431342

			# Restart multiple instances
			$ kraft cloud instance restart my-instance-431342 my-instance-other-2313
		`),
		Long: heredoc.Doc(`
			Restart instance(s) on Unikraft Cloud.
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

func (opts *RestartOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token, &opts.AllowInsecure)
	if err != nil {
		return fmt.Errorf("could not populate Metro and Token: %w", err)
	}

	return nil
}

func (opts *RestartOptions) Run(ctx context.Context, args []string) error {
	return Restart(ctx, opts, args...)
}

// Restart Unikraft Cloud instance(s).
func Restart(ctx context.Context, opts *RestartOptions, args ...string) error {
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

	if err := stop.Stop(ctx, &stop.StopOptions{
		All:    opts.All,
		Auth:   opts.Auth,
		Client: opts.Client,
		Force:  opts.Force,
		Metro:  opts.Metro,
		Token:  opts.Token,
		Wait:   opts.Wait,
	}, args...); err != nil {
		return fmt.Errorf("could not stop instance: %w", err)
	}

	return start.Start(ctx, &start.StartOptions{
		All:    opts.All,
		Auth:   opts.Auth,
		Client: opts.Client,
		Metro:  opts.Metro,
		Token:  opts.Token,
		Wait:   opts.Wait,
	}, args...)
}

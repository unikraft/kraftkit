// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package stop

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

type StopOptions struct {
	Auth         *config.AuthConfig    `noattribute:"true"`
	Client       kraftcloud.KraftCloud `noattribute:"true"`
	Wait         time.Duration         `local:"true" long:"wait" short:"w" usage:"Timeout for the instance to stop (ms/s/m/h)"`
	DrainTimeout time.Duration         `local:"true" long:"drain-timeout" short:"d" usage:"Time to wait for the instance to drain all connections before it is stopped (ms/s/m/h)"`
	All          bool                  `long:"all" short:"a" usage:"Stop all instances"`
	Force        bool                  `long:"force" short:"f" usage:"Force stop the instance(s)"`
	Metro        string                `noattribute:"true"`
	Token        string                `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StopOptions{}, cobra.Command{
		Short:   "Stop instances",
		Use:     "stop [FLAGS] [UUID|NAME [UUID|NAME]...]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"st"},
		Example: heredoc.Doc(`
			# Stop an instance by UUID
			$ kraft cloud instance stop 77d0316a-fbbe-488d-8618-5bf7a612477a

			# Stop an instance by name
			$ kraft cloud instance stop my-instance-431342

			# Stop multiple instances
			$ kraft cloud instance stop my-instance-431342 my-instance-other-2313

			# Stop all instances
			$ kraft cloud instance stop --all

			# Stop an instanace by name and wait for connections to drain for 5s
			$ kraft cloud instance stop --wait 5s my-instance-431342
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

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *StopOptions) Run(ctx context.Context, args []string) error {
	return Stop(ctx, opts, args...)
}

// Stop KraftCloud instance(s).
func Stop(ctx context.Context, opts *StopOptions, args ...string) error {
	var err error

	if opts.DrainTimeout != 0 && opts.Wait != 0 {
		return fmt.Errorf("drain-timeout and wait flags are mutually exclusive")
	}

	if opts.DrainTimeout != 0 && opts.Wait == 0 {
		opts.Wait = opts.DrainTimeout
		log.G(ctx).Warnf("drain timeout is deprecated, use wait instead")
	}

	if opts.Wait < time.Millisecond && opts.Wait != 0 {
		return fmt.Errorf("drain wait timeout must be at least 1ms")
	}

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

	timeout := int(opts.Wait.Milliseconds())

	if opts.All {
		instListResp, err := opts.Client.Instances().WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}

		instList, err := instListResp.AllOrErr()
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}
		if len(instList) == 0 {
			log.G(ctx).Info("no instances to stop")
			return nil
		}

		log.G(ctx).Infof("stopping %d instance(s)", len(instList))

		uuids := make([]string, 0, len(instList))
		for _, instItem := range instList {
			uuids = append(uuids, instItem.UUID)
		}

		if _, err := opts.Client.Instances().WithMetro(opts.Metro).Stop(ctx, timeout, opts.Force, uuids...); err != nil {
			return fmt.Errorf("stopping %d instance(s): %w", len(uuids), err)
		}

		return nil
	}

	log.G(ctx).Infof("stopping %d instance(s)", len(args))

	stopResp, err := opts.Client.Instances().WithMetro(opts.Metro).Stop(ctx, timeout, opts.Force, args...)
	if err != nil {
		return fmt.Errorf("stopping %d instance(s): %w", len(args), err)
	}
	if _, err = stopResp.AllOrErr(); err != nil {
		return fmt.Errorf("stopping %d instance(s): %w", len(args), err)
	}

	return nil
}

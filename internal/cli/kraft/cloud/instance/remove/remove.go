// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstances "sdk.kraft.cloud/instances"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type RemoveOptions struct {
	Auth    *config.AuthConfig    `noattribute:"true"`
	Client  kraftcloud.KraftCloud `noattribute:"true"`
	All     bool                  `long:"all" short:"a" usage:"Remove all instances"`
	Stopped bool                  `long:"stopped" short:"s" usage:"Remove all stopped instances"`
	Metro   string                `noattribute:"true"`
	Token   string                `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Remove instances",
		Use:     "remove [FLAGS] [UUID|NAME [UUID|NAME]...]",
		Aliases: []string{"del", "delete", "rm"},
		Args:    cobra.ArbitraryArgs,
		Example: heredoc.Doc(`
			# Remove an instance by UUID
			$ kraft cloud instance remove fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Remove an instance by name
			$ kraft cloud instance remove my-instance-431342

			# Remove multiple instances
			$ kraft cloud instance remove my-instance-431342 my-instance-other-2313

			# Remove all instances
			$ kraft cloud instance remove --all

			# Remove all stopped instances
			$ kraft cloud instance remove --stopped
		`),
		Long: heredoc.Doc(`
			Remove a KraftCloud instance.
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

func (opts *RemoveOptions) Pre(cmd *cobra.Command, args []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	return Remove(ctx, opts, args...)
}

// Remove KraftCloud instance(s).
func Remove(ctx context.Context, opts *RemoveOptions, args ...string) error {
	var err error

	if !opts.Stopped && !opts.All && len(args) == 0 {
		return fmt.Errorf("either specify an instance name or UUID, or use the --all flag")
	}
	if opts.Stopped && opts.All {
		return fmt.Errorf("cannot use --stopped and --all together")
	}
	if opts.Stopped && len(args) > 0 {
		return fmt.Errorf("cannot specify instances and use --stopped together")
	}
	if opts.All && len(args) > 0 {
		return fmt.Errorf("cannot specify instances and use --all together")
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

	if opts.All || opts.Stopped {
		instListResp, err := opts.Client.Instances().WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}

		if len(instListResp.Data.Entries) == 0 {
			log.G(ctx).Info("no instances found")
			return nil
		}

		uuids := make([]string, 0, len(instListResp.Data.Entries))
		for _, instItem := range instListResp.Data.Entries {
			uuids = append(uuids, instItem.UUID)
		}

		if opts.Stopped {
			instInfosResp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, uuids...)
			if err != nil {
				return fmt.Errorf("could not get instances: %w", err)
			}

			var stoppedUuids []string
			for _, instInfo := range instInfosResp.Data.Entries {
				if kcinstances.State(instInfo.State) == kcinstances.StateStopped {
					stoppedUuids = append(stoppedUuids, instInfo.UUID)
				}
			}
			if len(stoppedUuids) == 0 {
				return nil
			}

			uuids = stoppedUuids
		}

		log.G(ctx).Infof("removing %d instance(s)", len(uuids))

		if _, err := opts.Client.Instances().WithMetro(opts.Metro).Delete(ctx, uuids...); err != nil {
			return fmt.Errorf("removing %d instance(s): %w", len(uuids), err)
		}

		return nil
	}

	log.G(ctx).Infof("removing %d instance(s)", len(args))

	resp, err := opts.Client.Instances().WithMetro(opts.Metro).Delete(ctx, args...)
	if err != nil {
		return fmt.Errorf("removing %d instance(s): %w", len(args), err)
	}
	if _, err := resp.AllOrErr(); err != nil {
		return fmt.Errorf("removing %d instance(s): %w", len(args), err)
	}

	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

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

type RemoveOptions struct {
	All    bool                  `long:"all" usage:"Remove all services"`
	Auth   *config.AuthConfig    `noattribute:"true"`
	Client kraftcloud.KraftCloud `noattribute:"true"`
	Metro  string                `noattribute:"true"`
	Token  string                `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Delete a service group",
		Use:     "remove [FLAGS] [NAME|UUID [NAME|UUID]...]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"delete", "del", "rm"},
		Long:    "Delete a service group.",
		Example: heredoc.Doc(`
			# Remove a service group from your account by UUID.
			$ kraft cloud service remove fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Remove a service group from your account by name.
			$ kraft cloud service remove my-service-group

			# Remove multiple service groups from your account.
			$ kraft cloud service remove my-service-group my-other-service-group

			# Remove all service groups from your account.
			$ kraft cloud service remove --all
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-svc",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *RemoveOptions) Pre(cmd *cobra.Command, args []string) error {
	if !opts.All && len(args) == 0 {
		return fmt.Errorf("either specify an instance name or UUID, or use the --all flag")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	return Remove(ctx, opts, args...)
}

func Remove(ctx context.Context, opts *RemoveOptions, args ...string) error {
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

	if opts.All {
		sgListResp, err := opts.Client.Services().WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("listing service groups: %w", err)
		}

		if len(sgListResp.Data.Entries) == 0 {
			log.G(ctx).Info("no service groups found")
			return nil
		}

		uuids := make([]string, 0, len(sgListResp.Data.Entries))
		for _, sgItem := range sgListResp.Data.Entries {
			uuids = append(uuids, sgItem.UUID)
		}

		log.G(ctx).Infof("removing %d service group(s)", len(uuids))

		if _, err := opts.Client.Services().WithMetro(opts.Metro).Delete(ctx, uuids...); err != nil {
			return fmt.Errorf("removing %d service group(s): %w", len(uuids), err)
		}

		return nil
	}

	log.G(ctx).Infof("removing %d service group(s)", len(args))

	resp, err := opts.Client.Services().WithMetro(opts.Metro).Delete(ctx, args...)
	if err != nil {
		return fmt.Errorf("removing %d service group(s): %w", len(args), err)
	}
	if _, err := resp.AllOrErr(); err != nil {
		return fmt.Errorf("removing %d service group(s): %w", len(args), err)
	}

	return nil
}

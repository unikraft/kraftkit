// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kraftcloudservices "sdk.kraft.cloud/services"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type RemoveOptions struct {
	All    bool                               `long:"all" usage:"Remove all services"`
	Auth   *config.AuthConfig                 `noattribute:"true"`
	Client kraftcloudservices.ServicesService `noattribute:"true"`
	Metro  string                             `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Delete a service group",
		Use:     "rm [FLAGS] NAME|UUID",
		Aliases: []string{"delete", "del", "remove"},
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

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfigFromContext(ctx)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewServicesClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	if opts.All {
		groups, err := opts.Client.WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not get list of all instances: %w", err)
		}

		for _, group := range groups {
			log.G(ctx).Infof("removing %s (%s)", group.Name, group.UUID)

			if err := opts.Client.WithMetro(opts.Metro).DeleteByUUID(ctx, group.UUID); err != nil {
				log.G(ctx).Errorf("could not delete service group: %s", err.Error())
			}
		}
	}

	for _, arg := range args {
		if utils.IsUUID(arg) {
			err = opts.Client.WithMetro(opts.Metro).DeleteByUUID(ctx, arg)
		} else {
			err = opts.Client.WithMetro(opts.Metro).DeleteByName(ctx, arg)
		}
		if err != nil {
			return fmt.Errorf("could not delete service group: %w", err)
		}

		log.G(ctx).Infof("removing %s", arg)
	}

	return err
}

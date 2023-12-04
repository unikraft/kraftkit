// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package detach

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kraftcloudvolumes "sdk.kraft.cloud/volumes"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
)

type DetachOptions struct {
	Auth   *config.AuthConfig               `noattribute:"true"`
	Client kraftcloudvolumes.VolumesService `noattribute:"true"`

	metro string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&DetachOptions{}, cobra.Command{
		Short:   "Detach a volume from an instance",
		Use:     "detach [FLAGS] UUID",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"separate", "unmount"},
		Example: heredoc.Doc(`
		# Detach a volume from an instance
		$ kraft cloud volume detach UUID
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-vol",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *DetachOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.metro = cmd.Flag("metro").Value.String()
	if opts.metro == "" {
		opts.metro = os.Getenv("KRAFTCLOUD_METRO")
	}
	if opts.metro == "" {
		return fmt.Errorf("kraftcloud metro is unset")
	}

	log.G(cmd.Context()).WithField("metro", opts.metro).Debug("using")
	return nil
}

func (opts *DetachOptions) Run(ctx context.Context, args []string) error {
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfigFromContext(ctx)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewVolumesClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	var volume *kraftcloudvolumes.Volume

	if utils.IsUUID(args[0]) {
		volume, err = opts.Client.WithMetro(opts.metro).DetachByUUID(ctx, args[0])
	} else {
		volume, err = opts.Client.WithMetro(opts.metro).DetachByName(ctx, args[0])
	}
	if err != nil {
		return fmt.Errorf("could not create volume: %w", err)
	}

	_, err = fmt.Fprintln(iostreams.G(ctx).Out, volume.UUID)
	return err
}

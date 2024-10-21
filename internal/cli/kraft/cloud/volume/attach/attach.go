// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package attach

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	cloud "sdk.kraft.cloud"
	ukcvolumes "sdk.kraft.cloud/volumes"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/iostreams"
)

type AttachOptions struct {
	At       string                    `long:"at" usage:"The path the volume should be mounted to"`
	Auth     *config.AuthConfig        `noattribute:"true"`
	Client   ukcvolumes.VolumesService `noattribute:"true"`
	ReadOnly bool                      `long:"read-only" short:"r" usage:"Mount the volume read-only"`
	To       string                    `long:"to" usage:"The instance the volume should be attached to"`

	metro string
	token string
}

// Attach a UnikraftCloud persistent volume to an instance.
func Attach(ctx context.Context, opts *AttachOptions, args ...string) (*ukcvolumes.AttachResponseItem, error) {
	var err error

	if opts == nil {
		opts = &AttachOptions{}
	}

	if opts.At == "" {
		return nil, fmt.Errorf("required to set the destination instance")
	}

	if opts.To == "" {
		return nil, fmt.Errorf("required to set the destination path in the instance")
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetUnikraftCloudAuthConfig(ctx, opts.token)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = cloud.NewVolumesClient(
			cloud.WithToken(config.GetUnikraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	attachResp, err := opts.Client.WithMetro(opts.metro).Attach(ctx, args[0], opts.To, opts.At, opts.ReadOnly)
	if err != nil {
		return nil, fmt.Errorf("attaching volume %s: %w", args[0], err)
	}
	attach, err := attachResp.FirstOrErr()
	if err != nil {
		return nil, fmt.Errorf("attaching volume %s: %w", args[0], err)
	}

	return attach, nil
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&AttachOptions{}, cobra.Command{
		Short:   "Attach a persistent volume to an instance",
		Use:     "attach [FLAGS] UUID|NAME",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"at"},
		Example: heredoc.Doc(`
			# Attach the volume data to the instance nginx to the path /mnt/data
			$ kraft cloud vol attach data --to nginx --at /mnt/data

			# Attach a volume to the instance nginx to the path /mnt/data by UUID in read-only mode
			$ kraft cloud volume at 77d0316a-fbbe-488d-8618-5bf7a612477a --to nginx --at /mnt/data -r
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-vol",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *AttachOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.metro, &opts.token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *AttachOptions) Run(ctx context.Context, args []string) error {
	volume, err := Attach(ctx, opts, args...)
	if err != nil {
		return fmt.Errorf("could not attach volume: %w", err)
	}

	_, err = fmt.Fprintln(iostreams.G(ctx).Out, volume.UUID)
	return err
}

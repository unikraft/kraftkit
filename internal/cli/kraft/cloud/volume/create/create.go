// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kraftcloudvolumes "sdk.kraft.cloud/volumes"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/iostreams"
)

type CreateOptions struct {
	Auth   *config.AuthConfig               `noattribute:"true"`
	Client kraftcloudvolumes.VolumesService `noattribute:"true"`
	Metro  string                           `noattribute:"true"`
	Name   string                           `local:"true" size:"name" short:"n"`
	SizeMB int                              `local:"true" long:"size" short:"s" usage:"Size in MB"`
	Token  string                           `noattribute:"true"`
}

// Create a KraftCloud persistent volume.
func Create(ctx context.Context, opts *CreateOptions, args ...string) (*kraftcloudvolumes.Volume, error) {
	var err error

	if opts == nil {
		opts = &CreateOptions{}
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfigFromContext(ctx, opts.Token)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewVolumesClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	return opts.Client.WithMetro(opts.Metro).Create(ctx, opts.Name, opts.SizeMB)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short:   "Create a persistent volume",
		Use:     "create [FLAGS]",
		Args:    cobra.NoArgs,
		Aliases: []string{"crt"},
		Long: heredoc.Doc(`
			Create a new persistent volume.
		`),
		Example: heredoc.Doc(`
			# Create a new persistent 100MiB volume named "my-volume"
			$ kraft cloud volume create --size 100 --name my-volume
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

func (opts *CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	if opts.SizeMB == 0 {
		return fmt.Errorf("must specify --size flag")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	volume, err := Create(ctx, opts, args...)
	if err != nil {
		return fmt.Errorf("could not create volume: %w", err)
	}

	_, err = fmt.Fprintln(iostreams.G(ctx).Out, volume.UUID)
	return err
}

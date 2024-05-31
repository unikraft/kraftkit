// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import (
	"context"
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/resource"

	kraftcloud "sdk.kraft.cloud"
	kcvolumes "sdk.kraft.cloud/volumes"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/iostreams"
)

type CreateOptions struct {
	Auth   *config.AuthConfig       `noattribute:"true"`
	Client kcvolumes.VolumesService `noattribute:"true"`
	Metro  string                   `noattribute:"true"`
	Name   string                   `local:"true" size:"name" short:"n"`
	Size   string                   `local:"true" long:"size" short:"s" usage:"Size (MiB increments)"`
	Token  string                   `noattribute:"true"`
}

// Create a KraftCloud persistent volume.
func Create(ctx context.Context, opts *CreateOptions) (*kcvolumes.CreateResponseItem, error) {
	var err error

	if opts == nil {
		opts = &CreateOptions{}
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewVolumesClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	if _, err := strconv.ParseUint(opts.Size, 10, 64); err == nil {
		opts.Size = fmt.Sprintf("%sMi", opts.Size)
	}

	qty, err := resource.ParseQuantity(opts.Size)
	if err != nil {
		return nil, fmt.Errorf("could not parse size quantity: %w", err)
	}

	if qty.Value() < 1024*1024 {
		return nil, fmt.Errorf("size must be at least 1Mi")
	}

	// Convert to MiB
	sizeMB := int(qty.Value() / (1024 * 1024))

	createResp, err := opts.Client.WithMetro(opts.Metro).Create(ctx, opts.Name, sizeMB)
	if err != nil {
		return nil, fmt.Errorf("creating volume: %w", err)
	}
	create, err := createResp.FirstOrErr()
	if err != nil {
		return nil, fmt.Errorf("creating volume: %w", err)
	}

	return create, nil
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
	if opts.Size == "" {
		return fmt.Errorf("must specify --size flag")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, _ []string) error {
	volume, err := Create(ctx, opts)
	if err != nil {
		return fmt.Errorf("could not create volume: %w", err)
	}

	_, err = fmt.Fprintln(iostreams.G(ctx).Out, volume.UUID)
	return err
}

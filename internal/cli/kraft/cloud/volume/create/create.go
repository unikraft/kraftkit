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
)

type CreateOptions struct {
	AllowInsecure bool                     `noattribute:"true"`
	Auth          *config.AuthConfig       `noattribute:"true"`
	Client        kcvolumes.VolumesService `noattribute:"true"`
	Metro         string                   `noattribute:"true"`
	Name          string                   `local:"true" long:"name" short:"n" usage:"Name of the volume"`
	Size          string                   `local:"true" long:"size" short:"s" usage:"Size (MiB increments or suffixes like Mi, Gi, etc.)"`
	From          string                   `local:"true" long:"from" short:"f" usage:"Name or UUID of the template to create from"`
	Output        string                   `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list,raw" default:"table"`
	Token         string                   `noattribute:"true"`
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
			kraftcloud.WithAllowInsecure(opts.AllowInsecure),
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	req := &kcvolumes.CreateRequest{}

	if opts.Name != "" {
		req.Name = &opts.Name
	}

	if opts.Size != "" {
		if _, err := strconv.ParseUint(opts.Size, 10, 64); err == nil {
			opts.Size = fmt.Sprintf("%sMi", opts.Size)
		}

		qty, err := resource.ParseQuantity(opts.Size)
		if err != nil {
			return nil, fmt.Errorf("could not parse size quantity: %w", err)
		}

		if qty.Value() < 1024*1024 && qty.Value() != 0 {
			return nil, fmt.Errorf("size must be at least 1Mi")
		}

		// Convert to MiB
		sizeMB := int(qty.Value() / (1024 * 1024))
		req.SizeMb = &sizeMB
	}

	if opts.From != "" {
		if utils.IsUUID(opts.From) {
			req.Template = &kcvolumes.CreateRequestTemplate{
				UUID: &opts.From,
			}
		} else {
			req.Template = &kcvolumes.CreateRequestTemplate{
				Name: &opts.From,
			}
		}
	}

	createResp, err := opts.Client.WithMetro(opts.Metro).Create(ctx, req)
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

			# Create a new persistent 10MiB volume with a random name
			$ kraft cloud volume create --size 10Mi

			# Create a new persistent volume named "my-volume" by cloning an existing template "existing-template"
			$ kraft cloud volume create --from existing-template --name my-volume
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-volume",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	if opts.Size != "" && opts.From != "" {
		return fmt.Errorf("cannot specify both 'size' and template 'from'")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token, &opts.AllowInsecure)
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

	resp, err := opts.Client.WithMetro(opts.Metro).Get(ctx, volume.UUID)
	if err != nil {
		return fmt.Errorf("could not get volume %s: %w", volume.UUID, err)
	}

	return utils.PrintVolumes(ctx, opts.Output, *resp)
}

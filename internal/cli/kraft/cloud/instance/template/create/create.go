// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

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
	"kraftkit.sh/iostreams"
)

type CreateOptions struct {
	AllowInsecure bool                         `noattribute:"true"`
	Auth          *config.AuthConfig           `noattribute:"true"`
	Client        kcinstances.InstancesService `noattribute:"true"`
	Metro         string                       `noattribute:"true"`
	Token         string                       `noattribute:"true"`
}

// Create a Unikraft Cloud instance (snapshot) template.
func Create(ctx context.Context, opts *CreateOptions, args []string) ([]kcinstances.TemplateCreateResponseItem, error) {
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
		opts.Client = kraftcloud.NewInstancesClient(
			kraftcloud.WithAllowInsecure(opts.AllowInsecure),
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	createResp, err := opts.Client.WithMetro(opts.Metro).CreateTemplate(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("creating instance template: %w", err)
	}
	create, err := createResp.AllOrErr()
	if err != nil {
		return nil, fmt.Errorf("creating instance template: %w", err)
	}

	return create, nil
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short:   "Create instance template(s)",
		Use:     "create [FLAGS] NAME|UUID[,NAME|UUID...]",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"crt"},
		Long: heredoc.Doc(`
			Create a new persistent instance template.
		`),
		Example: heredoc.Doc(`
			# Create a new instance template from "my-instance" instance
			$ kraft cloud instance template create my-instance

			# Create two new instance templates from "my-instance" and "my-other-instance" instances
			$ kraft cloud instance template create my-instance my-other-instance
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-instance-template",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token, &opts.AllowInsecure)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	templates, err := Create(ctx, opts, args)
	if err != nil {
		return fmt.Errorf("could not create instance template(s): %w", err)
	}

	for _, template := range templates {
		_, err = fmt.Fprintln(iostreams.G(ctx).Out, template.UUID)
	}

	return err
}

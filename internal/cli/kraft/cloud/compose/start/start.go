// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package start

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/start"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type StartOptions struct {
	Auth        *config.AuthConfig    `noattribute:"true"`
	Client      kraftcloud.KraftCloud `noattribute:"true"`
	Composefile string                `noattribute:"true"`
	Metro       string                `noattribute:"true"`
	Project     *compose.Project      `noattribute:"true"`
	Token       string                `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StartOptions{}, cobra.Command{
		Short:   "Start a KraftCloud deployment",
		Use:     "start [FLAGS] [COMPONENT]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"str"},
		Long: heredoc.Doc(`
			Start an existing KraftCloud deployment.
		`),
		Example: heredoc.Doc(`
			# Stop an existing KraftCloud deployment fully.
			$ kraft cloud compose start

			# Stop an existing KraftCloud deployment with two specific components.
			$ kraft cloud compose start nginx component2
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-compose",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *StartOptions) Pre(cmd *cobra.Command, args []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if cmd.Flag("file").Changed {
		opts.Composefile = cmd.Flag("file").Value.String()
	}

	return nil
}

func (opts *StartOptions) Run(ctx context.Context, args []string) error {
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

	if opts.Project == nil {
		workdir, err := os.Getwd()
		if err != nil {
			return err
		}

		opts.Project, err = compose.NewProjectFromComposeFile(ctx, workdir, opts.Composefile)
		if err != nil {
			return err
		}
	}

	if err := opts.Project.Validate(ctx); err != nil {
		return err
	}

	// if no services are specified, start all services
	if len(args) == 0 {
		for service := range opts.Project.Services {
			args = append(args, service)
		}
	}

	var instances []string

	for _, serviceName := range args {
		service, ok := opts.Project.Services[serviceName]
		if !ok {
			return fmt.Errorf("service '%s' not found", serviceName)
		}

		instances = append(instances, service.Name)
	}

	return start.Start(ctx, &start.StartOptions{
		Auth:   opts.Auth,
		Client: opts.Client,
		Metro:  opts.Metro,
		Token:  opts.Token,
	}, instances...)
}

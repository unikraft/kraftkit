// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package ps

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	cloud "sdk.kraft.cloud"
	ukcclient "sdk.kraft.cloud/client"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type PsOptions struct {
	Auth        *config.AuthConfig `noattribute:"true"`
	Client      cloud.KraftCloud   `noattribute:"true"`
	Composefile string             `noattribute:"true"`
	Metro       string             `noattribute:"true"`
	Output      string             `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	Project     *compose.Project   `noattribute:"true"`
	Token       string             `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PsOptions{}, cobra.Command{
		Short: "List the active services of Unikraft Cloud Compose project",
		Use:   "ps [FLAGS]",
		Args:  cobra.NoArgs,
		Example: heredoc.Doc(`
			# List all active services
			$ kraft cloud compose ps
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-compose",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *PsOptions) Pre(cmd *cobra.Command, args []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *PsOptions) Run(ctx context.Context, args []string) error {
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetUnikraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = cloud.NewClient(
			cloud.WithToken(config.GetUnikraftCloudTokenAuthConfig(*opts.Auth)),
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

		name := strings.ReplaceAll(fmt.Sprintf("%s-%s", opts.Project.Name, service.Name), "_", "-")
		if cname := service.ContainerName; len(cname) > 0 {
			name = cname
		}

		instances = append(instances, name)
	}

	instancesResp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, instances...)
	if err != nil {
		return fmt.Errorf("getting instances: %w", err)
	}

	if len(instances) == 0 {
		return fmt.Errorf("no instances found")
	}

	for i, instance := range instancesResp.Data.Entries {
		if instance.Error != nil && *instance.Error == ukcclient.APIHTTPErrorNotFound {
			instancesResp.Data.Entries[i].Message = "not deployed"
		}
	}

	return utils.PrintInstances(ctx, opts.Output, *instancesResp)
}

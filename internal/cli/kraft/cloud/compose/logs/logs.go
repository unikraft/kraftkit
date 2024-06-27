// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package logs

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/logs"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type LogsOptions struct {
	Auth        *config.AuthConfig    `noattribute:"true"`
	Client      kraftcloud.KraftCloud `noattribute:"true"`
	Composefile string                `noattribute:"true"`
	Follow      bool                  `long:"follow" short:"f" usage:"Follow log output"`
	Metro       string                `noattribute:"true"`
	Output      string                `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	Project     *compose.Project      `noattribute:"true"`
	Tail        int                   `long:"tail" short:"t" usage:"Number of lines to show from the end of the logs" default:"-1"`
	Token       string                `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&LogsOptions{}, cobra.Command{
		Short:   "Log the services in a KraftCloud compose project deployment",
		Use:     "log [FLAGS]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"logs", "l"},
		Long: heredoc.Doc(`
		Log the services in a KraftCloud deployment.
		`),
		Example: heredoc.Doc(`
			# Log the services in a KraftCloud deployment.
			$ kraft cloud compose log

			# Log a service in a KraftCloud deployment.
			$ kraft cloud compose log nginx
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

func (opts *LogsOptions) Pre(cmd *cobra.Command, args []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if cmd.Flag("file").Changed {
		opts.Composefile = cmd.Flag("file").Value.String()
	}

	return nil
}

func (opts *LogsOptions) Run(ctx context.Context, args []string) error {
	return Logs(ctx, opts, args...)
}

func Logs(ctx context.Context, opts *LogsOptions, args ...string) error {
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

	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	if opts.Project == nil {
		opts.Project, err = compose.NewProjectFromComposeFile(ctx, workdir, opts.Composefile)
		if err != nil {
			return err
		}
	}

	if err := opts.Project.Validate(ctx); err != nil {
		return err
	}

	// If no services are specified, start all services.
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

	if len(instances) == 0 {
		return fmt.Errorf("no instances found")
	}

	return logs.Log(ctx, &logs.LogOptions{
		Auth:   opts.Auth,
		Client: opts.Client,
		Follow: opts.Follow,
		Metro:  opts.Metro,
		Tail:   opts.Tail,
	}, instances...)
}

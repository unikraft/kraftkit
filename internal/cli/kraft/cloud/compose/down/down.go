// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package down

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstances "sdk.kraft.cloud/instances"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	kcremove "kraftkit.sh/internal/cli/kraft/cloud/instance/remove"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type DownOptions struct {
	Auth   *config.AuthConfig           `noattribute:"true"`
	Client kcinstances.InstancesService `noattribute:"true"`
	Metro  string                       `noattribute:"true"`
	Token  string                       `noattribute:"true"`

	project     *compose.Project
	composeFile string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&DownOptions{}, cobra.Command{
		Short:   "Stop a KraftCloud deployment",
		Use:     "down [FLAGS] [COMPONENT]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"d"},
		Long: heredoc.Doc(`
			Stop a KraftCloud deployment.
		`),
		Example: heredoc.Doc(`
			# Stop a KraftCloud deployment fully.
			$ kraft cloud compose down

			# Stop a KraftCloud deployment with two specific components.
			$ kraft cloud compose down nginx component2
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

func (opts *DownOptions) Pre(cmd *cobra.Command, args []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *DownOptions) Run(ctx context.Context, args []string) error {
	var err error

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = kraftcloud.NewInstancesClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	opts.project, err = compose.NewProjectFromComposeFile(ctx, workdir, opts.composeFile)
	if err != nil {
		return err
	}

	if err := opts.project.Validate(ctx); err != nil {
		return err
	}

	// if no services are specified, stop all services
	if len(args) == 0 {
		for _, service := range opts.project.Services {
			args = append(args, strings.SplitN(service.Name, "-", 2)[1])
		}
	}

	var services []string
	for _, service := range opts.project.Services {
		for _, requestedService := range args {
			if service.Name != opts.project.Name+"-"+requestedService {
				continue
			}
			instanceResp, err := opts.Client.WithMetro(opts.Metro).Get(ctx, service.Name)
			if err != nil {
				return fmt.Errorf("getting instance %s: %w", service.Name, err)
			}
			instance, err := instanceResp.FirstOrErr()
			if err != nil {
				return fmt.Errorf("getting instance %s: %w", service.Name, err)
			}
			if instance.State == string(kcinstances.StateStopped) ||
				instance.State == string(kcinstances.StateStopping) ||
				instance.State == string(kcinstances.StateDraining) {
				log.G(ctx).WithField("service", service.Name).Info("service already stopped")
				continue
			}

			services = append(services, service.Name)
		}
	}

	if len(services) == 0 {
		return fmt.Errorf("no services to stop")
	}

	// if service is running, remove it
	stopOpts := kcremove.RemoveOptions{
		Output: "list",
		Metro:  opts.Metro,
		Token:  opts.Token,
	}
	return stopOpts.Run(ctx, services)
}

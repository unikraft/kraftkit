// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package down

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type DownOptions struct {
	Auth        *config.AuthConfig    `noattribute:"true"`
	Client      kraftcloud.KraftCloud `noattribute:"true"`
	Composefile string                `noattribute:"true"`
	Metro       string                `noattribute:"true"`
	Project     *compose.Project      `noattribute:"true"`
	Token       string                `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&DownOptions{}, cobra.Command{
		Short:   "Stop and remove the services in a Unikraft Cloud Compose project deployment",
		Use:     "down [FLAGS] [COMPONENT]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"d"},
		Example: heredoc.Doc(`
			# Stop a deployment and remove all instances, services and volumes.
			$ kraft cloud compose down

			# Stop and remove two specific instances.
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

	var instances []string

	// If no services are specified, remove all services.
	if len(args) == 0 {
		for _, service := range opts.Project.Services {
			name := strings.ReplaceAll(fmt.Sprintf("%s-%s", opts.Project.Name, service.Name), "_", "-")
			if cname := service.ContainerName; len(cname) > 0 {
				name = cname
			}

			instances = append(instances, name)
		}
	} else {
		for _, arg := range args {
			service, ok := opts.Project.Services[arg]
			if !ok {
				return fmt.Errorf("service '%s' not found", arg)
			}

			instances = append(instances, service.Name)
		}
	}

	var errGroup []error

	instResp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, instances...)
	if err != nil {
		return fmt.Errorf("getting instances: %w", err)
	}

	insts, err := instResp.AllOrErr()
	if err != nil {
		return fmt.Errorf("getting instances: %w", err)
	}

	instances = []string{}

	for _, instance := range insts {
		if instance.Message != "" {
			log.G(ctx).Error(instance.Message)
			continue
		}

		instances = append(instances, instance.Name)
	}

	if len(instances) > 0 {
		log.G(ctx).Infof("stopping %d instance(s)", len(instances))

		if _, err := opts.Client.Instances().WithMetro(opts.Metro).Delete(ctx, instances...); err != nil {
			errGroup = append(errGroup, fmt.Errorf("removing instances: %w", err))
		}
	}

	return errors.Join(errGroup...)
}

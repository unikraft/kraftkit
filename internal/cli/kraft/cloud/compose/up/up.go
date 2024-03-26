// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package up

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
	kccreate "kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	kcstart "kraftkit.sh/internal/cli/kraft/cloud/instance/start"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type UpOptions struct {
	Auth   *config.AuthConfig           `noattribute:"true"`
	Client kcinstances.InstancesService `noattribute:"true"`
	Metro  string                       `noattribute:"true"`
	Token  string                       `noattribute:"true"`

	project     *compose.Project
	composeFile string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&UpOptions{}, cobra.Command{
		Short:   "Start a KraftCloud deployment",
		Use:     "up [FLAGS] [COMPONENT]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"u"},
		Long: heredoc.Doc(`
			Start a KraftCloud deployment.
		`),
		Example: heredoc.Doc(`
			# Start a KraftCloud deployment fully.
			$ kraft cloud compose up

			# Start a KraftCloud deployment with two specific components.
			$ kraft cloud compose up nginx component2
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

func (opts *UpOptions) Pre(cmd *cobra.Command, args []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *UpOptions) Run(ctx context.Context, args []string) error {
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

	// if no services are specified, start all services
	if len(args) == 0 {
		for _, service := range opts.project.Services {
			args = append(args, strings.SplitN(service.Name, "-", 2)[1])
		}
	}

	for _, service := range opts.project.Services {
		for _, requestedService := range args {
			if service.Name != opts.project.Name+"-"+requestedService {
				continue
			}
			log.G(ctx).WithField("service", service.Name).Info("starting service")

			instanceResp, err := opts.Client.WithMetro(opts.Metro).Get(ctx, service.Name)
			if err != nil {
				return fmt.Errorf("getting instance %s: %w", service.Name, err)
			}
			if instance, err := instanceResp.FirstOrErr(); err != nil {
				if !strings.Contains(err.Error(), "(code=8)") {
					return fmt.Errorf("getting instance %s: %w", service.Name, err)
				}

				// if the service does not exist, create it
				start := true
				memory := int(service.MemLimit / 1024 / 1024)
				var ports []string
				for _, port := range service.Ports {
					if port.Published == "443" {
						ports = append(ports, fmt.Sprintf("%s:%d/http+tls", port.Published, port.Target))
					} else if port.Target == 443 {
						ports = append(ports, fmt.Sprintf("%s:%d/http+redirect", port.Published, port.Target))
					} else {
						ports = append(ports, fmt.Sprintf("%s:%d/tls", port.Published, port.Target))
					}
				}

				createOpts := kccreate.CreateOptions{
					Output: "list",
					Name:   service.Name,
					Memory: memory,
					Ports:  ports,
					Start:  start,
					Metro:  opts.Metro,
					Auth:   opts.Auth,
				}

				err := createOpts.Run(ctx, []string{service.Image})
				if err != nil {
					return err
				}
			} else {
				if instance.State == string(kcinstances.StateRunning) ||
					instance.State == string(kcinstances.StateStarting) ||
					instance.State == string(kcinstances.StateStandby) {
					log.G(ctx).WithField("service", service.Name).Info("service already running")
					continue
				}

				// if service is not running, start it
				startOpts := kcstart.StartOptions{
					Output: "list",
					Metro:  opts.Metro,
					Token:  opts.Token,
				}
				err := startOpts.Run(ctx, []string{service.Name})
				if err != nil {
					return err
				}
			}
		}
	}

	return err
}

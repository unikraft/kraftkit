// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kraftcloudinstances "sdk.kraft.cloud/instances"
	kraftcloudservices "sdk.kraft.cloud/services"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type CreateOptions struct {
	Auth     *config.AuthConfig                   `noattribute:"true"`
	Client   kraftcloudinstances.InstancesService `noattribute:"true"`
	Env      []string                             `local:"true" long:"env" short:"e" usage:"Environmental variables"`
	Memory   int64                                `local:"true" long:"memory" short:"M" usage:"Specify the amount of memory to allocate"`
	Metro    string                               `noattribute:"true"`
	Name     string                               `local:"true" long:"name" short:"n" usage:"Specify the name of the package"`
	Output   string                               `local:"true" long:"output" short:"o" usage:"Set output format" default:"table"`
	Ports    []string                             `local:"true" long:"port" short:"p" usage:"Specify the port mapping between external to internal"`
	Replicas int                                  `local:"true" long:"replicas" short:"R" usage:"Number of replicas of the instance" default:"1"`
	Start    bool                                 `local:"true" long:"start" short:"S" usage:"Immediately start the instance after creation"`
}

// Create a KraftCloud instance.
func Create(ctx context.Context, opts *CreateOptions, args ...string) (*kraftcloudinstances.Instance, error) {
	var err error

	if opts == nil {
		opts = &CreateOptions{}
	}

	image := args[0]

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudLoginFromContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}
	if opts.Client == nil {
		opts.Client = kraftcloud.NewInstancesClient(
			kraftcloud.WithToken(opts.Auth.Token),
		)
	}

	services := []kraftcloudservices.Service{}

	if len(opts.Ports) == 1 && strings.HasPrefix(opts.Ports[0], "443:") && strings.Count(opts.Ports[0], "/") == 0 {
		split := strings.Split(opts.Ports[0], ":")
		if len(split) != 2 {
			return nil, fmt.Errorf("malformed port expeted format EXTERNAL:INTERNAL[/HANDLER[,HANDLER...]]")
		}

		destPort, err := strconv.Atoi(split[1])
		if err != nil {
			return nil, fmt.Errorf("invalid external port: %w", err)
		}

		services = []kraftcloudservices.Service{
			{
				Port:            443,
				DestinationPort: destPort,
				Handlers: []kraftcloudservices.Handler{
					kraftcloudservices.HandlerHTTP,
					kraftcloudservices.HandlerTLS,
				},
			},
			{
				Port:            80,
				DestinationPort: 443,
				Handlers: []kraftcloudservices.Handler{
					kraftcloudservices.HandlerHTTP,
					kraftcloudservices.HandlerRedirect,
				},
			},
		}

	} else {
		for _, port := range opts.Ports {
			service := kraftcloudservices.Service{
				Handlers: []kraftcloudservices.Handler{},
			}

			if strings.ContainsRune(port, '/') {
				split := strings.Split(port, "/")
				if len(split) != 2 {
					return nil, fmt.Errorf("malformed port expeted format EXTERNAL:INTERNAL[/HANDLER[,HANDLER...]]")
				}

				for _, handler := range strings.Split(split[1], "+") {
					h := kraftcloudservices.Handler(handler)
					if !slices.Contains(kraftcloudservices.Handlers(), h) {
						return nil, fmt.Errorf("unknown handler: %s (choice of %v)", handler, kraftcloudservices.Handlers())
					}

					service.Handlers = append(service.Handlers, h)
				}

				port = split[0]
			}

			if strings.ContainsRune(port, ':') {
				ports := strings.Split(port, ":")
				if len(ports) != 2 {
					return nil, fmt.Errorf("invalid --port value expected --port EXTERNAL:INTERNAL")
				}

				service.Port, err = strconv.Atoi(ports[0])
				if err != nil {
					return nil, fmt.Errorf("invalid internal port: %w", err)
				}

				service.DestinationPort, err = strconv.Atoi(ports[1])
				if err != nil {
					return nil, fmt.Errorf("invalid external port: %w", err)
				}
			} else {
				port, err := strconv.Atoi(port)
				if err != nil {
					return nil, fmt.Errorf("could not parse port number: %w", err)
				}

				service.Port = port
				service.DestinationPort = port
			}

			services = append(services, service)
		}
	}

	envs := make(map[string]string)
	for _, env := range opts.Env {
		if strings.ContainsRune(env, '=') {
			split := strings.SplitN(env, "=", 2)
			envs[split[0]] = split[1]
		} else {
			envs[env] = os.Getenv(env)
		}
	}

	return opts.Client.WithMetro(opts.Metro).Create(ctx, kraftcloudinstances.CreateInstanceRequest{
		Image:     image,
		Args:      args[1:],
		MemoryMB:  opts.Memory,
		Services:  services,
		Autostart: opts.Start,
		Instances: opts.Replicas,
		Env:       envs,
	})
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short:   "Create an instance",
		Use:     "create [FLAGS] IMAGE [-- ARGS]",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"new"},
		Example: heredoc.Doc(`
			# Create a hello world instance
			$ kraft cloud instance create -M 64 unikraft.org/helloworld:latest

			# Create a new NGINX instance in Frankfurt and start it immediately.  Map the external
			# port 443 to the internal port 80 which the application listens on.
			$ kraft cloud --metro fra0 instance create \
				--start \
				--port 443:80 \
				unikraft.io/official/nginx:latest

			# This command is the same as above, however using the more elaborate port expression.
			# This is because in fact we need need to accept TLS and HTTP connections and redirect
			# port 80 to port 443.  The above example exists only as a shortcut for what is written
			# below:
			$ kraft cloud --metro fra0 instance create \
				--start \
				--port 443:80/http+tls \
				--port 80:443/http+redirect \
				unikraft.io/official/nginx:latest
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-instance",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	opts.Metro = cmd.Flag("metro").Value.String()
	if opts.Metro == "" {
		opts.Metro = os.Getenv("KRAFTCLOUD_METRO")
	}
	if opts.Metro == "" {
		return fmt.Errorf("kraftcloud metro is unset")
	}
	log.G(cmd.Context()).WithField("metro", opts.Metro).Debug("using")
	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	instance, err := Create(ctx, opts, args...)
	if err != nil {
		return err
	}

	return utils.PrintInstances(ctx, opts.Output, *instance)
}

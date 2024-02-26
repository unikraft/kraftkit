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
	kraftcloudservices "sdk.kraft.cloud/services"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
)

type CreateOptions struct {
	Auth      *config.AuthConfig                 `noattribute:"true"`
	Client    kraftcloudservices.ServicesService `noattribute:"true"`
	FQDN      string                             `local:"true" long:"fqdn" short:"d" usage:"Specify the fully qualified domain name of the service"`
	SubDomain string                             `local:"true" long:"subdomain" short:"s" usage:"Set the subdomain to use when creating the service"`
	Metro     string                             `noattribute:"true"`
	Name      string                             `local:"true" long:"name" short:"n" usage:"Specify the name of the package"`
	Output    string                             `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
}

// Create a KraftCloud instance.
func Create(ctx context.Context, opts *CreateOptions, args ...string) (*kraftcloudservices.ServiceGroup, error) {
	var err error

	if opts == nil {
		opts = &CreateOptions{}
	}

	if len(opts.SubDomain) > 0 && len(opts.FQDN) > 0 {
		return nil, fmt.Errorf("the `--subdomain|-s` option is mutually exclusive with `--fqdn|--domain|-d`")
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfigFromContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}
	if opts.Client == nil {
		opts.Client = kraftcloud.NewServicesClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	services := []kraftcloudservices.Service{}

	if len(args) == 1 && strings.HasPrefix(args[0], "443:") && strings.Count(args[0], "/") == 0 {
		split := strings.Split(args[0], ":")
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
		for _, port := range args {
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

	req := kraftcloudservices.ServiceCreateRequest{
		Name:     opts.Name,
		Services: services,
	}

	if len(opts.SubDomain) > 0 {
		req.DNSName = strings.TrimSuffix(opts.SubDomain, ".")
	} else if len(opts.FQDN) > 0 {
		if !strings.HasSuffix(".", opts.FQDN) {
			opts.FQDN += "."
		}

		req.DNSName = opts.FQDN
	}

	return opts.Client.WithMetro(opts.Metro).Create(ctx, req)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short:   "Create a service group",
		Use:     "create [FLAGS] EXTERNAL:INTERNAL[/HANDLER[+HANDLER...]]",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"new"},
		Long:    "Create a service group.",
		Example: heredoc.Doc(`
			# Create a service group with a single service listening on port 443 named "my-service"
			$ kraft cloud service create --name my-service 443:8080/http+tls
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-svc",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.Flags().String(
		"domain",
		"",
		"Alias for --fqdn|-d",
	)

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

	domain := cmd.Flag("domain").Value.String()
	if len(domain) > 0 && len(opts.FQDN) > 0 {
		return fmt.Errorf("cannot use --domain and --fqdn together")
	} else if len(domain) > 0 && len(opts.FQDN) == 0 {
		opts.FQDN = domain
	}

	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	serviceGroup, err := Create(ctx, opts, args...)
	if err != nil {
		return err
	}

	return utils.PrintServiceGroups(ctx, opts.Output, *serviceGroup)
}

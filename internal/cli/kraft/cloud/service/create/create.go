// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type CreateOptions struct {
	Auth      *config.AuthConfig         `noattribute:"true"`
	Client    kcservices.ServicesService `noattribute:"true"`
	FQDN      string                     `local:"true" long:"fqdn" short:"d" usage:"Specify the fully qualified domain name of the service"`
	SubDomain string                     `local:"true" long:"subdomain" short:"s" usage:"Set the subdomain to use when creating the service"`
	Metro     string                     `noattribute:"true"`
	Name      string                     `local:"true" long:"name" short:"n" usage:"Specify the name of the service"`
	Output    string                     `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	Token     string                     `noattribute:"true"`
}

// Create a KraftCloud instance.
func Create(ctx context.Context, opts *CreateOptions, args ...string) (*kcservices.CreateResponseItem, error) {
	var err error

	if opts == nil {
		opts = &CreateOptions{}
	}

	if len(opts.SubDomain) > 0 && len(opts.FQDN) > 0 {
		return nil, fmt.Errorf("the `--subdomain|-s` option is mutually exclusive with `--fqdn|--domain|-d`")
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}
	if opts.Client == nil {
		opts.Client = kraftcloud.NewServicesClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	var services []kcservices.CreateRequestService

	if len(args) == 1 && strings.HasPrefix(args[0], "443:") && strings.Count(args[0], "/") == 0 {
		split := strings.Split(args[0], ":")
		if len(split) != 2 {
			return nil, fmt.Errorf("malformed port expeted format EXTERNAL:INTERNAL[/HANDLER[,HANDLER...]]")
		}

		destPort, err := strconv.Atoi(split[1])
		if err != nil {
			return nil, fmt.Errorf("invalid external port: %w", err)
		}

		services = make([]kcservices.CreateRequestService, 0, 2)

		port443 := 443
		services = append(services,
			kcservices.CreateRequestService{
				Port:            443,
				DestinationPort: &destPort,
				Handlers: []kcservices.Handler{
					kcservices.HandlerHTTP,
					kcservices.HandlerTLS,
				},
			},
			kcservices.CreateRequestService{
				Port:            80,
				DestinationPort: &port443,
				Handlers: []kcservices.Handler{
					kcservices.HandlerHTTP,
					kcservices.HandlerRedirect,
				},
			},
		)
	} else {
		services = make([]kcservices.CreateRequestService, 0, len(args))

		for _, port := range args {
			var service kcservices.CreateRequestService

			if strings.ContainsRune(port, '/') {
				split := strings.Split(port, "/")
				if len(split) != 2 {
					return nil, fmt.Errorf("malformed port, expected format EXTERNAL:INTERNAL[/HANDLER[,HANDLER...]]")
				}

				for _, handler := range strings.Split(split[1], "+") {
					h := kcservices.Handler(handler)
					if !slices.Contains(kcservices.Handlers(), h) {
						return nil, fmt.Errorf("unknown handler: %s (choice of %v)", handler, kcservices.Handlers())
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

				dstPort, err := strconv.Atoi(ports[1])
				if err != nil {
					return nil, fmt.Errorf("invalid external port: %w", err)
				}
				service.DestinationPort = &dstPort
			} else {
				port, err := strconv.Atoi(port)
				if err != nil {
					return nil, fmt.Errorf("could not parse port number: %w", err)
				}

				service.Port = port
				service.DestinationPort = &port
			}

			services = append(services, service)
		}
	}

	req := kcservices.CreateRequest{
		Services: services,
	}
	if opts.Name != "" {
		req.Name = &opts.Name
	}

	if len(opts.SubDomain) > 0 {
		dnsName := strings.TrimSuffix(opts.SubDomain, ".")
		req.Domains = []kcservices.CreateRequestDomain{{
			Name: dnsName,
		}}
	} else if len(opts.FQDN) > 0 {
		if !strings.HasSuffix(".", opts.FQDN) {
			opts.FQDN += "."
		}

		req.Domains = []kcservices.CreateRequestDomain{{
			Name: opts.FQDN,
		}}
	}

	sgResp, err := opts.Client.WithMetro(opts.Metro).Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("creating service: %w", err)
	}
	sg, err := sgResp.FirstOrErr()
	if err != nil {
		return nil, fmt.Errorf("creating service: %w", err)
	}

	return sg, nil
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
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	domain := cmd.Flag("domain").Value.String()
	if len(domain) > 0 && len(opts.FQDN) > 0 {
		return fmt.Errorf("cannot use --domain and --fqdn together")
	} else if len(domain) > 0 && len(opts.FQDN) == 0 {
		opts.FQDN = domain
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	newSg, err := Create(ctx, opts, args...)
	if err != nil {
		return fmt.Errorf("creating service group: %w", err)
	}

	sgResp, err := opts.Client.WithMetro(opts.Metro).GetByUUID(ctx, newSg.UUID)
	if err != nil {
		return fmt.Errorf("getting details of service group %s: %w", newSg.UUID, err)
	}
	sg, err := sgResp.FirstOrErr()
	if err != nil {
		return fmt.Errorf("getting details of service group %s: %w", newSg.UUID, err)
	}

	return utils.PrintServiceGroups(ctx, opts.Output, *sg)
}

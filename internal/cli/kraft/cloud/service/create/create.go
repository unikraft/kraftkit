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

	cloud "sdk.kraft.cloud"
	ukcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
)

type CreateOptions struct {
	Auth        *config.AuthConfig          `noattribute:"true"`
	Client      ukcservices.ServicesService `noattribute:"true"`
	Certificate []string                    `local:"true" long:"certificate" short:"c" usage:"Set the certificates to use for the service"`
	Domain      []string                    `local:"true" long:"domain" short:"d" usage:"Specify the domain names of the service"`
	SubDomain   []string                    `local:"true" long:"subdomain" short:"s" usage:"Set the subdomains to use when creating the service"`
	SoftLimit   uint                        `local:"true" long:"soft-limit" short:"l" usage:"Set the soft limit for the service"`
	HardLimit   uint                        `local:"true" long:"hard-limit" short:"L" usage:"Set the hard limit for the service"`
	Metro       string                      `noattribute:"true"`
	Name        string                      `local:"true" long:"name" short:"n" usage:"Specify the name of the service"`
	Output      string                      `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	Token       string                      `noattribute:"true"`
}

// Create a UnikraftCloud instance.
func Create(ctx context.Context, opts *CreateOptions, args ...string) (*ukcservices.CreateResponseItem, error) {
	var err error

	if opts == nil {
		opts = &CreateOptions{}
	}

	// if len(opts.SubDomain) > 0 && len(opts.FQDN) > 0 {
	// 	return nil, fmt.Errorf("the `--subdomain|-s` option is mutually exclusive with `--fqdn|--domain|-d`")
	// }

	if len(opts.Domain) > 0 && len(opts.Certificate) > 0 && len(opts.Domain) != len(opts.Certificate) {
		return nil, fmt.Errorf("number of certificates does not match number of domains")
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetUnikraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}
	if opts.Client == nil {
		opts.Client = cloud.NewServicesClient(
			cloud.WithToken(config.GetUnikraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	var services []ukcservices.CreateRequestService

	if len(args) == 1 && strings.HasPrefix(args[0], "443:") && strings.Count(args[0], "/") == 0 {
		split := strings.Split(args[0], ":")
		if len(split) != 2 {
			return nil, fmt.Errorf("malformed port expeted format EXTERNAL:INTERNAL[/HANDLER]")
		}

		destPort, err := strconv.Atoi(split[1])
		if err != nil {
			return nil, fmt.Errorf("invalid external port: %w", err)
		}

		services = make([]ukcservices.CreateRequestService, 0, 2)

		port443 := 443
		services = append(services,
			ukcservices.CreateRequestService{
				Port:            443,
				DestinationPort: &destPort,
				Handlers: []ukcservices.Handler{
					ukcservices.HandlerHTTP,
					ukcservices.HandlerTLS,
				},
			},
			ukcservices.CreateRequestService{
				Port:            80,
				DestinationPort: &port443,
				Handlers: []ukcservices.Handler{
					ukcservices.HandlerHTTP,
					ukcservices.HandlerRedirect,
				},
			},
		)
	} else {
		services = make([]ukcservices.CreateRequestService, 0, len(args))

		for _, port := range args {
			var service ukcservices.CreateRequestService

			if strings.ContainsRune(port, '/') {
				split := strings.Split(port, "/")
				if len(split) != 2 {
					return nil, fmt.Errorf("malformed port, expected format EXTERNAL:INTERNAL[/HANDLER]")
				}

				for _, handler := range strings.Split(split[1], "+") {
					h := ukcservices.Handler(handler)
					if !slices.Contains(ukcservices.Handlers(), h) {
						return nil, fmt.Errorf("unknown handler: %s (choice of %v)", handler, ukcservices.Handlers())
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

	req := ukcservices.CreateRequest{
		Services: services,
		Domains:  []ukcservices.CreateRequestDomain{},
	}
	if opts.Name != "" {
		req.Name = &opts.Name
	}
	if opts.SoftLimit > 0 {
		sl := int(opts.SoftLimit)
		req.SoftLimit = &sl
	}
	if opts.HardLimit > 0 {
		hl := int(opts.HardLimit)
		req.HardLimit = &hl
	}

	for i, fqdn := range opts.Domain {
		if !strings.HasSuffix(".", fqdn) {
			fqdn += "."
		}

		domainCreate := ukcservices.CreateRequestDomain{
			Name: fqdn,
		}

		if len(opts.Certificate) > i {
			if utils.IsUUID(opts.Certificate[i]) {
				domainCreate.Certificate = &ukcservices.CreateRequestDomainCertificate{
					UUID: opts.Certificate[i],
				}
			} else {
				domainCreate.Certificate = &ukcservices.CreateRequestDomainCertificate{
					Name: opts.Certificate[i],
				}
			}
		}

		req.Domains = append(req.Domains, domainCreate)
	}

	for _, subdomain := range opts.SubDomain {
		dnsName := strings.TrimSuffix(subdomain, ".")
		req.Domains = append(req.Domains, ukcservices.CreateRequestDomain{
			Name: dnsName,
		})
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
		Short:   "Create a service",
		Use:     "create [FLAGS] EXTERNAL:INTERNAL[/HANDLER]",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"new"},
		Long:    "Create a service.",
		Example: heredoc.Doc(`
			# Create a service with a single service listening on port 443 named "my-service"
			$ kraft cloud service create -n my-service 443:8080/http+tls
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-svc",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	if opts.SoftLimit != 0 && opts.HardLimit != 0 && opts.SoftLimit > opts.HardLimit {
		return fmt.Errorf("soft limit must be less than or equal to the hard limit")
	}

	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	newSg, err := Create(ctx, opts, args...)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}

	resp, err := opts.Client.WithMetro(opts.Metro).Get(ctx, newSg.UUID)
	if err != nil {
		return fmt.Errorf("getting details of service %s: %w", newSg.UUID, err)
	}

	return utils.PrintServices(ctx, opts.Output, *resp)
}

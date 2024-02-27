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
	Auth                   *config.AuthConfig    `noattribute:"true"`
	Client                 kraftcloud.KraftCloud `noattribute:"true"`
	Env                    []string              `local:"true" long:"env" short:"e" usage:"Environmental variables"`
	Features               []string              `local:"true" long:"feature" short:"f" usage:"List of features to enable"`
	FQDN                   string                `local:"true" long:"fqdn" short:"d" usage:"The Fully Qualified Domain Name to use for the service"`
	Image                  string                `noattribute:"true"`
	Memory                 int64                 `local:"true" long:"memory" short:"M" usage:"Specify the amount of memory to allocate (MiB)"`
	Metro                  string                `noattribute:"true"`
	Name                   string                `local:"true" long:"name" short:"n" usage:"Specify the name of the package"`
	Output                 string                `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	Ports                  []string              `local:"true" long:"port" short:"p" usage:"Specify the port mapping between external to internal"`
	Replicas               int                   `local:"true" long:"replicas" short:"R" usage:"Number of replicas of the instance" default:"0"`
	ServiceGroupNameOrUUID string                `local:"true" long:"service-group" short:"g" usage:"Attach this instance to an existing service group"`
	Start                  bool                  `local:"true" long:"start" short:"S" usage:"Immediately start the instance after creation"`
	ScaleToZero            bool                  `local:"true" long:"scale-to-zero" short:"0" usage:"Scale the instance to zero after deployment"`
	SubDomain              string                `local:"true" long:"subdomain" short:"s" usage:"Set the subdomain to use when creating the service"`
	Token                  string                `noattribute:"true"`
	Volumes                []string              `local:"true" long:"volumes" short:"v" usage:"List of volumes to attach instance to"`
}

// Create a KraftCloud instance.
func Create(ctx context.Context, opts *CreateOptions, args ...string) (*kraftcloudinstances.Instance, error) {
	var err error

	if opts == nil {
		opts = &CreateOptions{}
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return nil, fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}
	if opts.Client == nil {
		opts.Client = kraftcloud.NewClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	var features []kraftcloudinstances.InstanceFeature

	if opts.ScaleToZero {
		features = append(features, kraftcloudinstances.FeatureScaleToZero)
	}

	for _, feature := range opts.Features {
		formattedFeature := kraftcloudinstances.InstanceFeature(feature)
		if !slices.Contains(features, formattedFeature) {
			features = append(features, formattedFeature)
		}
	}

	req := kraftcloudinstances.CreateInstanceRequest{
		Args:      args,
		Autostart: opts.Start,
		Env:       make(map[string]string),
		Features:  features,
		Image:     opts.Image,
		MemoryMB:  opts.Memory,
		Name:      opts.Name,
		Replicas:  opts.Replicas,
		Volumes:   []kraftcloudinstances.CreateInstanceVolumeRequest{},
	}

	for _, vol := range opts.Volumes {
		split := strings.Split(vol, ":")
		if len(split) < 2 || len(split) > 3 {
			return nil, fmt.Errorf("invalid syntax for -v|--volume: expected VOLUME:PATH[:ro]")
		}
		volume := kraftcloudinstances.CreateInstanceVolumeRequest{
			At: split[1],
		}
		if utils.IsUUID(split[0]) {
			volume.UUID = split[0]
		} else {
			volume.Name = split[0]
		}
		if len(split) == 3 && split[2] == "ro" {
			volume.ReadOnly = true
		} else {
			volume.ReadOnly = false
		}

		req.Volumes = append(req.Volumes, volume)
	}

	var serviceGroup *kraftcloudservices.ServiceGroup

	if len(opts.ServiceGroupNameOrUUID) > 0 {
		if utils.IsUUID(opts.ServiceGroupNameOrUUID) {
			serviceGroup, err = opts.Client.Services().WithMetro(opts.Metro).GetByUUID(ctx, opts.ServiceGroupNameOrUUID)
		} else {
			serviceGroup, err = opts.Client.Services().WithMetro(opts.Metro).GetByName(ctx, opts.ServiceGroupNameOrUUID)
		}
		if err != nil {
			return nil, fmt.Errorf("could not use service '%s': %w", opts.ServiceGroupNameOrUUID, err)
		}

		log.G(ctx).
			WithField("uuid", serviceGroup.UUID).
			Debug("using service group")

		req.ServiceGroup = &kraftcloudinstances.CreateInstanceServiceGroupRequest{
			UUID: serviceGroup.UUID,
		}
	}

	// TODO(nderjung): This should eventually be possible, when the KraftCloud API
	// supports updating service groups.
	if len(opts.ServiceGroupNameOrUUID) > 0 && len(opts.Ports) > 0 {
		return nil, fmt.Errorf("cannot use existing --service-group|-g and define new --port|-p")
	}

	services := []kraftcloudservices.Service{}

	if len(opts.Ports) == 1 && strings.HasPrefix(opts.Ports[0], "443:") && strings.Count(opts.Ports[0], "/") == 0 {
		split := strings.Split(opts.Ports[0], ":")
		if len(split) != 2 {
			return nil, fmt.Errorf("malformed port expected format EXTERNAL:INTERNAL[/HANDLER[,HANDLER...]]")
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
					return nil, fmt.Errorf("malformed port expected format EXTERNAL:INTERNAL[/HANDLER[,HANDLER...]]")
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

	if len(opts.ServiceGroupNameOrUUID) == 0 {
		if len(services) > 0 {
			req.ServiceGroup = &kraftcloudinstances.CreateInstanceServiceGroupRequest{
				Services: services,
			}
		}
		if len(opts.SubDomain) > 0 {
			if req.ServiceGroup == nil {
				req.ServiceGroup = &kraftcloudinstances.CreateInstanceServiceGroupRequest{
					DNSName:  strings.TrimSuffix(opts.SubDomain, "."),
					Services: services,
				}
			} else {
				req.ServiceGroup.DNSName = strings.TrimSuffix(opts.SubDomain, ".")
			}
		} else if len(opts.FQDN) > 0 {
			if !strings.HasSuffix(".", opts.FQDN) {
				opts.FQDN += "."
			}

			if req.ServiceGroup == nil {
				req.ServiceGroup = &kraftcloudinstances.CreateInstanceServiceGroupRequest{
					DNSName:  opts.FQDN,
					Services: services,
				}
			} else {
				req.ServiceGroup.DNSName = opts.FQDN
			}
		}
	}

	for _, env := range opts.Env {
		if strings.ContainsRune(env, '=') {
			split := strings.SplitN(env, "=", 2)
			req.Env[split[0]] = split[1]
		} else {
			req.Env[env] = os.Getenv(env)
		}
	}

	instance, err := opts.Client.Instances().WithMetro(opts.Metro).Create(ctx, req)
	if err != nil {
		return nil, err
	}

	// Due to a limitation of the API, hydrate the object.
	instance, err = opts.Client.Instances().WithMetro(opts.Metro).GetByUUID(ctx, instance.UUID)
	if err != nil {
		return instance, err
	}

	if instance.ServiceGroup != nil && len(instance.ServiceGroup.UUID) > 0 {
		serviceGroup, err := opts.Client.Services().WithMetro(opts.Metro).GetByUUID(ctx, instance.ServiceGroup.UUID)
		if err != nil {
			return nil, err
		}

		instance.ServiceGroup = serviceGroup
	}

	return instance, nil
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&CreateOptions{}, cobra.Command{
		Short:   "Create an instance",
		Use:     "create [FLAGS] IMAGE [-- ARGS]",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"new"},
		Example: heredoc.Doc(`
			# Create a new NGINX instance in Frankfurt and start it immediately. Map the external
			# port 443 to the internal port 8080 which the application listens on.
			$ kraft cloud --metro fra0 instance create \
				--start \
				--port 443:8080 \
				nginx:latest

			# This command is the same as above, however using the more elaborate port expression.
			# This is because in fact we need need to accept TLS and HTTP connections and redirect
			# port 8080 to port 443.  The above example exists only as a shortcut for what is written
			# below:
			$ kraft cloud --metro fra0 instance create \
				--start \
				--port 443:8080/http+tls \
				--port 80:443/http+redirect \
				nginx:latest

			# Attach two existing volumes to the vm, one read-write at /data
			# and another read-only at /config:
			$ kraft cloud --metro fra0 instance create \
				--start \
				--volume my-data-vol:/data \
				--volume my-config-vol:/config:ro \
				nginx:latest
		`),
		Long: heredoc.Doc(`
			Create an instance on KraftCloud from an image.
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-instance",
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

	log.G(cmd.Context()).WithField("metro", opts.Metro).Debug("using")
	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	opts.Image = args[0]

	instance, err := Create(ctx, opts, args[1:]...)
	if err != nil {
		return err
	}

	if opts.Output != "table" && opts.Output != "full" {
		return utils.PrintInstances(ctx, opts.Output, *instance)
	}
	utils.PrettyPrintInstance(ctx, instance, opts.Start)

	return nil
}

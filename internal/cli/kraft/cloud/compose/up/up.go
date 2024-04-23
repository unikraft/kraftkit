// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package up

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcclient "sdk.kraft.cloud/client"
	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"
	kcvolumes "sdk.kraft.cloud/volumes"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/build"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/logs"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/start"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
)

type UpOptions struct {
	Auth        *config.AuthConfig    `noattribute:"true"`
	Client      kraftcloud.KraftCloud `noattribute:"true"`
	Composefile string                `noattribute:"true"`
	Detach      bool                  `local:"true" long:"detach" short:"d" usage:"Run the services in the background"`
	Metro       string                `noattribute:"true"`
	NoStart     bool                  `noattribute:"true"`
	Project     *compose.Project      `noattribute:"true"`
	Token       string                `noattribute:"true"`
	Wait        time.Duration         `local:"true" long:"wait" short:"w" usage:"Timeout to wait for the instance to start (ms/s/m/h)"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&UpOptions{}, cobra.Command{
		Short:   "Deploy services in a compose project to KraftCloud",
		Use:     "up [FLAGS] [COMPONENT]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"u"},
		Long: heredoc.Docf(`
			Deploy services in a compose project to KraftCloud

			Use an existing %[1]sComposefile%[1]s or %[1]sdocker-compose.yaml%[1]s file to start a
			number of services as instances on KraftCloud.

			Note that this is an experimental command and not all attributes of the
			%[1]sComposefile%[1]s are supported nor are all flags identical.
		`, "`"),
		Example: heredoc.Doc(`
			# Start a KraftCloud deployment fully.
			$ kraft cloud compose up

			# Start a KraftCloud deployment with two specific components.
			$ kraft cloud compose up nginx component
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

	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	if cmd.Flag("file").Changed {
		opts.Composefile = cmd.Flag("file").Value.String()
	}

	return nil
}

func (opts *UpOptions) Run(ctx context.Context, args []string) error {
	return Up(ctx, opts, args...)
}

func Up(ctx context.Context, opts *UpOptions, args ...string) error {
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

	// If no services are specified, start all services.
	if len(args) == 0 {
		for service := range opts.Project.Services {
			args = append(args, service)
		}
	}

	// Build all services if the build flag is set.
	if err := build.Build(ctx, &build.BuildOptions{
		Auth:        opts.Auth,
		Composefile: opts.Composefile,
		Metro:       opts.Metro,
		Project:     opts.Project,
		Token:       opts.Token,
		Push:        true,
	}, args...); err != nil {
		return err
	}

	// Preemptively create all the service groups from the compose project's
	// supplied networks.
	svcResps, err := createServiceGroupsFromNetworks(ctx, opts, args...)
	if err != nil {
		return err
	}

	volResps, err := createVolumes(ctx, opts)
	if err != nil {
		return err
	}

	instResps := kcclient.ServiceResponse[kcinstances.GetResponseItem]{}

	for _, serviceName := range args {
		service, ok := opts.Project.Services[serviceName]
		if !ok {
			return fmt.Errorf("service '%s' not found", serviceName)
		}

		instResp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, service.Name)
		if err == nil && len(instResp.Data.Entries) == 1 && instResp.Data.Entries[0].Error == nil {
			instResps.Data.Entries = append(instResps.Data.Entries, instResp.Data.Entries...)
			log.G(ctx).WithField("name", service.Name).Info("service already exists")
			continue
		}

		// There is only 1 network per service, so we can safely iterate over the
		// networks and break after the first iteration.
		var network string
		for n := range service.Networks {
			network = n
			break
		}

		// Handle environmental variables.
		var env []string
		for k, v := range service.Environment {
			if *v == "" {
				env = append(env, fmt.Sprintf("%s=%s", k, os.Getenv(k)))
			} else {
				env = append(env, fmt.Sprintf("%s=%s", k, *v))
			}
		}

		// Handle the memory limit and reservation.  Since these two concepts do not
		// currently exist via the KraftCloud API, pick the limit if it is set as it
		// represents the maximum value, otherwise check if the reservation has been
		// set.
		var memory int
		if service.MemLimit > 0 {
			memory = int(service.MemLimit)
		} else if service.MemReservation > 0 {
			memory = int(service.MemReservation)
		}

		if service.Image == "" {
			user := strings.TrimSuffix(strings.TrimPrefix(opts.Auth.User, "robot$"), ".users.kraftcloud")
			service.Image = fmt.Sprintf(
				"index.unikraft.io/%s/%s:latest",
				user,
				strings.ReplaceAll(service.Name, "_", "-"),
			)
		}

		log.G(ctx).WithField("image", service.Image).Info("deploying")

		var serviceGroup string
		if data, ok := svcResps[network]; ok {
			serviceGroup = data.Data.Entries[0].UUID
		}

		var volumes []string
		for _, volume := range service.Volumes {
			vol, ok := volResps[volume.Source]
			if !ok {
				continue
			}

			volumes = append(volumes, fmt.Sprintf("%s:%s", vol.Data.Entries[0].UUID, volume.Target))
		}

		name := service.Name
		if cname := service.ContainerName; len(cname) > 0 {
			name = cname
		}

		instResp, _, err = create.Create(ctx, &create.CreateOptions{
			Auth:                   opts.Auth,
			Client:                 opts.Client,
			Env:                    env,
			Image:                  service.Image,
			Memory:                 uint(memory),
			Metro:                  opts.Metro,
			Name:                   name,
			ServiceGroupNameOrUUID: serviceGroup,
			Start:                  false,
			Token:                  opts.Token,
			WaitForImage:           true,
			Volumes:                volumes,
		}, service.Command...)
		if err != nil {
			return err
		}

		instResps.Data.Entries = append(instResps.Data.Entries, instResp.Data.Entries...)
	}

	var instances []string
	for _, inst := range instResps.Data.Entries {
		instances = append(instances, inst.Name)
	}

	if !opts.NoStart {
		// Start the instances together, separate from the previous create
		// invocation.
		if err := start.Start(ctx, &start.StartOptions{
			Auth:   opts.Auth,
			Client: opts.Client,
			Metro:  opts.Metro,
			Token:  opts.Token,
			Wait:   opts.Wait,
		}, instances...); err != nil {
			return fmt.Errorf("starting instances: %w", err)
		}
	}

	if opts.Detach {
		return utils.PrintInstances(ctx, "table", instResps)
	}

	return logs.Logs(ctx, &logs.LogOptions{
		Auth:   opts.Auth,
		Client: opts.Client,
		Metro:  opts.Metro,
		Follow: true,
		Tail:   -1,
	}, instances...)
}

// createVolumes is used to create volumes for each service in the compose
// project.  These volumes are used to persist data across instances.
func createVolumes(ctx context.Context, opts *UpOptions) (map[string]*kcclient.ServiceResponse[kcvolumes.GetResponseItem], error) {
	volResps := make(map[string]*kcclient.ServiceResponse[kcvolumes.GetResponseItem])

	for alias, volume := range opts.Project.Volumes {
		name := strings.ReplaceAll(volume.Name, "_", "-")

		volResp, err := opts.Client.Volumes().WithMetro(opts.Metro).Get(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("getting volume: %w", err)
		}

		vol, err := volResp.FirstOrErr()
		if err != nil && vol != nil && *vol.Error == kcclient.APIHTTPErrorNotFound {

			log.G(ctx).WithField("name", name).Info("creating volume")

			size := 64
			if sentry, ok := volume.DriverOpts["size"]; ok {
				parsed, err := humanize.ParseBytes(sentry)
				if err != nil {
					return nil, fmt.Errorf("parsing volume size: %w", err)
				}

				size = int(parsed) / 1024 / 1024
			}

			createResp, err := opts.Client.Volumes().WithMetro(opts.Metro).Create(ctx, name, size)
			if err != nil {
				return nil, fmt.Errorf("creating volume: %w", err)
			}

			vol, err := createResp.FirstOrErr()
			if err != nil {
				return nil, err
			}

			getResp, err := opts.Client.Volumes().WithMetro(opts.Metro).Get(ctx, vol.UUID)
			if err != nil {
				return nil, fmt.Errorf("creating volume: %w", err)
			}

			volResps[alias] = getResp
		} else if err != nil {
			return nil, err
		} else {
			log.G(ctx).Warnf("volume '%s' already exists as '%s'", volume.Name, name)
			volResps[alias] = volResp
		}
	}

	return volResps, nil
}

// createServiceGroupsFromNetworks is used to map each compose service's
// networks to a service group.  Since it is not possible to attach an instance
// to multiple service groups it also acts a checking mechanism to determine if
// the compose project's networks are valid with respect to the capabilities of
// the KraftCloud API.
func createServiceGroupsFromNetworks(ctx context.Context, opts *UpOptions, args ...string) (map[string]*kcclient.ServiceResponse[kcservices.GetResponseItem], error) {
	svcResps := make(map[string]*kcclient.ServiceResponse[kcservices.GetResponseItem])

	// First, create a map of networks which are used by individual services.
	// These represent service groups and we'll create them only if they don't
	// exist and use the port mapping of the individual service to hydrate the
	// create request for the service group.
	svcReqs := make(map[string]kcservices.CreateRequest)
	for alias, service := range opts.Project.Services {
		if !slices.Contains(args, alias) {
			continue
		}

		if len(service.Networks) > 1 {
			return nil, fmt.Errorf("service '%s' has more than one network attached which is not supported", alias)
		}

		for network := range service.Networks {
			// Skip the default network, in the situation where a possible alternative
			// network can be supplied, we prefer this.  Only later if we detect that
			// there are no networks associated with any of the services, we'll create
			// a service group with the default network.
			if network == "default" && len(service.Networks) > 1 {
				continue
			}

			if _, ok := svcReqs[network]; ok {
				continue
			}

			svcReqs[network] = kcservices.CreateRequest{}
		}
	}

	// If none of the compose services are associated with a network, create a new
	// service group for the default network.
	if len(svcReqs) == 0 {
		svcReqs["default"] = kcservices.CreateRequest{}
	}

	// Check each network to determine whether it exists as a service group.
	for alias, network := range opts.Project.Networks {
		service, ok := svcReqs[alias]
		if !ok {
			continue
		}

		service.Name = ptr(strings.ReplaceAll(network.Name, "_", "-"))
		svcReqs[alias] = service

		respSvc, err := opts.Client.Services().WithMetro(opts.Metro).Get(ctx, *service.Name)
		if err != nil {
			return nil, err
		}

		svc, err := respSvc.FirstOrErr()
		if err != nil && *svc.Error == kcclient.APIHTTPErrorNotFound {
			continue
		} else if err != nil {
			return nil, err
		}

		log.G(ctx).Warnf("network '%s' already exists as a service group '%s'", network.Name, *service.Name)
		delete(svcReqs, alias)
		svcResps[alias] = respSvc
	}

	// Iterate through each service and grab the associated port mappings.  This
	// will be used later to hydrate the service group create request.
	for alias, service := range opts.Project.Services {
		if !slices.Contains(args, alias) {
			continue
		}

		var services []kcservices.CreateRequestService
		for _, port := range service.Ports {
			if port.Protocol != "" && port.Protocol != "tls" && port.Protocol != "tcp" {
				return nil, fmt.Errorf("protocol '%s' is not supported", port.Protocol)
			}

			if port.Published == "443" {
				services = append(services,
					kcservices.CreateRequestService{
						Port:            443,
						DestinationPort: ptr(int(port.Target)),
						Handlers: []kcservices.Handler{
							kcservices.HandlerHTTP,
							kcservices.HandlerTLS,
						},
					},
					kcservices.CreateRequestService{
						Port:            80,
						DestinationPort: ptr(int(443)),
						Handlers: []kcservices.Handler{
							kcservices.HandlerHTTP,
							kcservices.HandlerRedirect,
						},
					},
				)
			} else {
				published, err := strconv.Atoi(port.Published)
				if err != nil {
					return nil, fmt.Errorf("invalid external port: %w", err)
				}

				services = append(services,
					kcservices.CreateRequestService{
						Port:            published,
						DestinationPort: ptr(int(port.Target)),
						Handlers: []kcservices.Handler{
							kcservices.HandlerTLS,
						},
					},
				)
			}
		}

		// Expose all ports (services) to networks (service groups).
		for alias, req := range svcReqs {
			req.Services = append(req.Services, services...)

			if len(service.DomainName) > 0 {
				// If the domain contains a period, it is a fully qualified domain name,
				// which means we should append a period to the end of the domain name
				// to ensure it is a valid domain name for the KraftCloud API.
				if strings.Contains(service.DomainName, ".") {
					service.DomainName += "."
				}

				req.Domains = []kcservices.CreateRequestDomain{{
					Name: service.DomainName,
				}}
			}

			svcReqs[alias] = req
		}
	}

	// Create all service groups.
	for alias, req := range svcReqs {
		if len(req.Services) == 0 {
			log.G(ctx).
				WithField("network", alias).
				Warn("no exposed ports: skipping service group creation")
			continue
		}

		log.G(ctx).WithField("network", *req.Name).Info("creating service group")

		createResp, err := opts.Client.Services().WithMetro(opts.Metro).Create(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("creating service group: %w", err)
		}

		svc, err := createResp.FirstOrErr()
		if err != nil {
			return nil, err
		}

		getResp, err := opts.Client.Services().WithMetro(opts.Metro).Get(ctx, svc.UUID)
		if err != nil {
			return nil, fmt.Errorf("creating service group: %w", err)
		}

		svcResps[alias] = getResp
	}

	return svcResps, nil
}

func ptr[T comparable](v T) *T { return &v }

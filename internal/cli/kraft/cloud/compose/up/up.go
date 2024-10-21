// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package up

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	cloud "sdk.kraft.cloud"
	ukcclient "sdk.kraft.cloud/client"
	ukcinstances "sdk.kraft.cloud/instances"
	ukcservices "sdk.kraft.cloud/services"
	ukcvolumes "sdk.kraft.cloud/volumes"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/build"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/get"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/logs"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/start"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
)

type UpOptions struct {
	Auth        *config.AuthConfig `noattribute:"true"`
	Client      cloud.KraftCloud   `noattribute:"true"`
	Composefile string             `noattribute:"true"`
	Detach      bool               `local:"true" long:"detach" short:"d" usage:"Run the services in the background"`
	Metro       string             `noattribute:"true"`
	NoStart     bool               `noattribute:"true"`
	NoBuild     bool               `local:"true" long:"no-build" usage:"Do not build the services before starting them"`
	Project     *compose.Project   `noattribute:"true"`
	Runtimes    []string           `long:"runtime" usage:"Alternative runtime to use when packaging a service"`
	Token       string             `noattribute:"true"`
	Wait        time.Duration      `local:"true" long:"wait" short:"w" usage:"Timeout to wait for the instance to start (ms/s/m/h)"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&UpOptions{}, cobra.Command{
		Short:   "Deploy services in a compose project to Unikraft Cloud",
		Use:     "up [FLAGS] [COMPONENT]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"u"},
		Long: heredoc.Docf(`
			Deploy services in a compose project to Unikraft Cloud

			Use an existing %[1]sComposefile%[1]s or %[1]sdocker-compose.yaml%[1]s file to start a
			number of services as instances on Unikraft Cloud.

			Note that this is an experimental command and not all attributes of the
			%[1]sComposefile%[1]s are supported nor are all flags identical.
		`, "`"),
		Example: heredoc.Doc(`
			# Build and deploy a Compose project on Unikraft Cloud.
			$ kraft cloud compose up

			# Build and deploy the nginx service from a Compose project on
			# Unikraft Cloud.
			$ kraft cloud compose up nginx component

			# (If applicable) Set or override a runtime for a particular service
			$ kraft cloud compose up --runtime app=base:latest
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-compose",
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
		opts.Auth, err = config.GetUnikraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}

	if opts.Client == nil {
		opts.Client = cloud.NewClient(
			cloud.WithToken(config.GetUnikraftCloudTokenAuthConfig(*opts.Auth)),
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

	userName := strings.TrimSuffix(
		strings.TrimPrefix(opts.Auth.User, "robot$"), ".users.kraftcloud",
	)

	if !opts.NoBuild {
		// Build all services if the build flag is set.
		if err := build.Build(ctx, &build.BuildOptions{
			Auth:        opts.Auth,
			Client:      opts.Client,
			Composefile: opts.Composefile,
			Metro:       opts.Metro,
			Project:     opts.Project,
			Runtimes:    opts.Runtimes,
			Token:       opts.Token,
			Push:        true,
		}, args...); err != nil {
			return err
		}
	}

	volResps, err := createVolumes(ctx, opts)
	if err != nil {
		return err
	}

	insts := []ukcinstances.GetResponseItem{}

	for _, serviceName := range args {
		service, ok := opts.Project.Services[serviceName]
		if !ok {
			return fmt.Errorf("service '%s' not found", serviceName)
		}

		var (
			userPkgName     string
			officialPkgName string
		)

		if service.Image != "" {
			if !strings.Contains(service.Image, ":") {
				service.Image += ":latest"
			}

			userPkgName = fmt.Sprintf(
				"%s/%s",
				userName,
				strings.ReplaceAll(service.Image, "_", "-"),
			)
			officialPkgName = strings.ReplaceAll(service.Image, "_", "-")
		} else {
			userPkgName = fmt.Sprintf(
				"%s/%s-%s:latest",
				userName,
				strings.ReplaceAll(opts.Project.Name, "_", "-"),
				strings.ReplaceAll(service.Name, "_", "-"),
			)
		}

		if exists, _ := opts.imageExists(ctx, officialPkgName); officialPkgName != "" && exists {
			// Override the image name if it is set with the new package name.
			service.Image = officialPkgName
			opts.Project.Services[serviceName] = service

		} else if exists, _ := opts.imageExists(ctx, userPkgName); userPkgName != "" && exists {
			// Override the image name if it is set with the new package name.
			service.Image = userPkgName
			opts.Project.Services[serviceName] = service

		} else if opts.NoBuild {
			return fmt.Errorf("image '%s' not found in the catalog", service.Image)
		}

		instResp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, service.Name)
		if err == nil {
			inst, err := instResp.FirstOrErr()
			if err == nil && inst != nil {
				insts = append(insts, *inst)
				log.G(ctx).WithField("name", service.Name).Info("service already exists")
				continue
			}
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
		// currently exist via the UnikraftCloud API, pick the limit if it is set as it
		// represents the maximum value, otherwise check if the reservation has been
		// set.
		var memory string
		if service.MemLimit > 0 {
			memory = fmt.Sprintf("%d", int(service.MemLimit)/1024/1024)
		} else if service.MemReservation > 0 {
			memory = fmt.Sprintf("%d", int(service.MemReservation)/1024/1024)
		}

		log.G(ctx).
			WithField("image", service.Image).
			Info("creating instance")

		var volumes []string
		for _, volume := range service.Volumes {
			volResp, ok := volResps[volume.Source]
			if !ok {
				continue
			}

			vol, err := volResp.FirstOrErr()
			if err != nil {
				continue
			}

			volumes = append(volumes, fmt.Sprintf("%s:%s", vol.UUID, volume.Target))
		}

		name := strings.ReplaceAll(fmt.Sprintf("%s-%s", opts.Project.Name, service.Name), "_", "-")
		if cname := service.ContainerName; len(cname) > 0 {
			name = cname
		}

		var services []ukcservices.CreateRequestService

		for _, port := range service.Ports {
			if port.Protocol != "" && port.Protocol != "tls" && port.Protocol != "tcp" {
				return fmt.Errorf("protocol '%s' is not supported", port.Protocol)
			}

			if port.Published == "443" {
				services = append(services,
					ukcservices.CreateRequestService{
						Port:            443,
						DestinationPort: ptr(int(port.Target)),
						Handlers: []ukcservices.Handler{
							ukcservices.HandlerHTTP,
							ukcservices.HandlerTLS,
						},
					},
					ukcservices.CreateRequestService{
						Port:            80,
						DestinationPort: ptr(int(443)),
						Handlers: []ukcservices.Handler{
							ukcservices.HandlerHTTP,
							ukcservices.HandlerRedirect,
						},
					},
				)
			} else {
				published, err := strconv.Atoi(port.Published)
				if err != nil {
					return fmt.Errorf("invalid external port: %w", err)
				}

				services = append(services,
					ukcservices.CreateRequestService{
						Port:            published,
						DestinationPort: ptr(int(port.Target)),
						Handlers: []ukcservices.Handler{
							ukcservices.HandlerTLS,
						},
					},
				)
			}
		}

		var domains []string
		if len(service.DomainName) > 0 {
			domains = []string{service.DomainName}
		}

		instResp, _, err = create.Create(ctx, &create.CreateOptions{
			Auth:         opts.Auth,
			Client:       opts.Client,
			Domain:       domains,
			Entrypoint:   service.Entrypoint,
			Env:          env,
			Image:        service.Image,
			Memory:       memory,
			Metro:        opts.Metro,
			Name:         name,
			Services:     services,
			Start:        false,
			Token:        opts.Token,
			WaitForImage: !opts.NoBuild,
			Volumes:      volumes,
		}, service.Command...)
		if err != nil {
			return fmt.Errorf("creating instance: %w", err)
		}

		inst, err := instResp.FirstOrErr()
		if err != nil || inst == nil {
			return fmt.Errorf("creating instance: %w", err)
		}

		insts = append(insts, *inst)
	}

	var instances []string
	for _, inst := range insts {
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
		if !opts.NoStart {
			return get.Get(ctx, &get.GetOptions{
				Auth:   opts.Auth,
				Client: opts.Client,
				Metro:  opts.Metro,
				Token:  opts.Token,
				Output: "table",
			}, instances...)
		} else {
			instResps := ukcclient.ServiceResponse[ukcinstances.GetResponseItem]{}
			instResps.Data.Entries = insts
			return utils.PrintInstances(ctx, "table", instResps)
		}
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
func createVolumes(ctx context.Context, opts *UpOptions) (map[string]*ukcclient.ServiceResponse[ukcvolumes.GetResponseItem], error) {
	volResps := make(map[string]*ukcclient.ServiceResponse[ukcvolumes.GetResponseItem])

	for alias, volume := range opts.Project.Volumes {
		name := strings.ReplaceAll(volume.Name, "_", "-")

		volResp, err := opts.Client.Volumes().WithMetro(opts.Metro).Get(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("getting volume: %w", err)
		}

		vol, err := volResp.FirstOrErr()
		if err != nil && vol != nil && *vol.Error == ukcclient.APIHTTPErrorNotFound {

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

func ptr[T comparable](v T) *T { return &v }

// imageExists checks if an image exists in the UnikraftCloud registry.
func (opts *UpOptions) imageExists(ctx context.Context, name string) (exists bool, err error) {
	if name == "" {
		return false, fmt.Errorf("image name is empty")
	}

	log.G(ctx).
		WithField("image", name).
		Trace("checking exists")

	imageResp, err := opts.Client.Images().Get(ctx, name)
	if err != nil {
		return false, err
	}

	image, err := imageResp.FirstOrErr()
	if err != nil {
		return false, err
	} else if image == nil {
		return false, nil
	}

	return true, nil
}

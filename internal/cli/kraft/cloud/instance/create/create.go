// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcclient "sdk.kraft.cloud/client"
	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/processtree"
)

type CreateOptions struct {
	Auth                   *config.AuthConfig    `noattribute:"true"`
	Client                 kraftcloud.KraftCloud `noattribute:"true"`
	Env                    []string              `local:"true" long:"env" short:"e" usage:"Environmental variables"`
	Features               []string              `local:"true" long:"feature" short:"f" usage:"List of features to enable"`
	FQDN                   string                `local:"true" long:"fqdn" short:"d" usage:"The Fully Qualified Domain Name to use for the service"`
	Image                  string                `noattribute:"true"`
	Memory                 int                   `local:"true" long:"memory" short:"M" usage:"Specify the amount of memory to allocate (MiB)"`
	Metro                  string                `noattribute:"true"`
	Name                   string                `local:"true" long:"name" short:"n" usage:"Specify the name of the instance"`
	Output                 string                `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	Ports                  []string              `local:"true" long:"port" short:"p" usage:"Specify the port mapping between external to internal"`
	Replicas               int                   `local:"true" long:"replicas" short:"R" usage:"Number of replicas of the instance" default:"0"`
	Rollout                bool                  `local:"true" long:"rollout" short:"r" usage:"Roll out the instance in a service group"`
	ServiceGroupNameOrUUID string                `local:"true" long:"service-group" short:"g" usage:"Attach this instance to an existing service group"`
	Start                  bool                  `local:"true" long:"start" short:"S" usage:"Immediately start the instance after creation"`
	ScaleToZero            bool                  `local:"true" long:"scale-to-zero" short:"0" usage:"Scale the instance to zero after deployment"`
	SubDomain              string                `local:"true" long:"subdomain" short:"s" usage:"Set the subdomain to use when creating the service"`
	Token                  string                `noattribute:"true"`
	Volumes                []string              `local:"true" long:"volumes" short:"v" usage:"List of volumes to attach instance to"`
	WaitForImage           bool                  `local:"true" long:"wait-for-image" short:"w" usage:"Wait for the image to be available before creating the instance"`
}

// Create a KraftCloud instance.
func Create(ctx context.Context, opts *CreateOptions, args ...string) (*kcclient.ServiceResponse[kcinstances.GetResponseItem], *kcservices.GetResponseItem, error) {
	var err error

	if opts == nil {
		opts = &CreateOptions{}
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetKraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return nil, nil, fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}
	if opts.Client == nil {
		opts.Client = kraftcloud.NewClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	if !strings.Contains(opts.Image, ":") {
		opts.Image += ":latest"
	}

	// Check if the image exists before creating the instance
	if opts.WaitForImage {
		imageClient := kraftcloud.NewClient(
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)

		paramodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(false),
				processtree.WithRenderer(
					log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
				),
				processtree.WithFailFast(true),
				processtree.WithHideOnSuccess(true),
				processtree.WithTimeout(60),
			},
			processtree.NewProcessTreeItem(
				"waiting for the image to be available",
				"",
				func(ctx context.Context) error {
					opts.Image = strings.TrimPrefix(opts.Image, "index.unikraft.io/")
					opts.Image = strings.TrimPrefix(opts.Image, "official/")

					for {
						imgs, err := imageClient.Images().WithMetro(opts.Metro).List(ctx)
						if err != nil {
							return fmt.Errorf("could not list images: %w", err)
						}

						for _, img := range imgs.Data.Entries {
							if strings.Contains(opts.Image, "@") {
								if img.Digest == opts.Image {
									return nil
								}
							} else {
								for _, tag := range img.Tags {
									if tag == opts.Image {
										return nil
									}
								}
							}
						}

						time.Sleep(time.Second)
					}
				},
			),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("could not start the process tree: %w", err)
		}

		err = paramodel.Start()
		if err != nil {
			return nil, nil, fmt.Errorf("could not wait for image be available: %w", err)
		}
	}

	var features []kcinstances.Feature

	if opts.ScaleToZero {
		features = append(features, kcinstances.FeatureScaleToZero)
	}

	for _, feature := range opts.Features {
		formattedFeature := kcinstances.Feature(feature)
		if !slices.Contains(features, formattedFeature) {
			features = append(features, formattedFeature)
		}
	}

	req := kcinstances.CreateRequest{
		Autostart: &opts.Start,
		Features:  features,
		Image:     opts.Image,
	}
	if opts.Name != "" {
		req.Name = &opts.Name
	}
	if len(args) > 0 {
		req.Args = args
	}
	if opts.Memory > 0 {
		req.MemoryMB = &opts.Memory
	}
	if opts.Replicas > 0 {
		req.Replicas = &opts.Replicas
	}

	for _, vol := range opts.Volumes {
		split := strings.Split(vol, ":")
		if len(split) < 2 || len(split) > 3 {
			return nil, nil, fmt.Errorf("invalid syntax for -v|--volume: expected VOLUME:PATH[:ro]")
		}
		volume := kcinstances.CreateRequestVolume{
			At: &split[1],
		}
		if utils.IsUUID(split[0]) {
			volume.UUID = &split[0]
		} else {
			volume.Name = &split[0]
		}
		if len(split) == 3 && split[2] == "ro" {
			trueVal := true
			volume.ReadOnly = &trueVal
		} else {
			falseVal := false
			volume.ReadOnly = &falseVal
		}

		req.Volumes = append(req.Volumes, volume)
	}

	var serviceGroup *kcservices.GetResponseItem

	if opts.ServiceGroupNameOrUUID != "" {
		resp, err := opts.Client.Services().WithMetro(opts.Metro).Get(ctx, opts.ServiceGroupNameOrUUID)
		if err != nil {
			return nil, nil, fmt.Errorf("could not use service %s: %w", opts.ServiceGroupNameOrUUID, err)
		}
		serviceGroup, err = resp.FirstOrErr()
		if err != nil {
			return nil, nil, fmt.Errorf("could not use service %s: %w", opts.ServiceGroupNameOrUUID, err)
		}

		log.G(ctx).
			WithField("uuid", serviceGroup.UUID).
			Debug("using service group")

		req.ServiceGroup = &kcinstances.CreateRequestServiceGroup{
			UUID: &serviceGroup.UUID,
		}
	}

	// TODO(nderjung): This should eventually be possible, when the KraftCloud API
	// supports updating service groups.
	if opts.ServiceGroupNameOrUUID != "" && len(opts.Ports) > 0 {
		return nil, nil, fmt.Errorf("cannot use existing --service-group|-g and define new --port|-p")
	}

	var services []kcservices.CreateRequestService

	if len(opts.Ports) == 1 && strings.HasPrefix(opts.Ports[0], "443:") && strings.Count(opts.Ports[0], "/") == 0 {
		split := strings.Split(opts.Ports[0], ":")
		if len(split) != 2 {
			return nil, nil, fmt.Errorf("malformed port expected format EXTERNAL:INTERNAL[/HANDLER[,HANDLER...]]")
		}

		destPort, err := strconv.Atoi(split[1])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid external port: %w", err)
		}

		port443 := 443
		services = []kcservices.CreateRequestService{
			{
				Port:            443,
				DestinationPort: &destPort,
				Handlers: []kcservices.Handler{
					kcservices.HandlerHTTP,
					kcservices.HandlerTLS,
				},
			},
			{
				Port:            80,
				DestinationPort: &port443,
				Handlers: []kcservices.Handler{
					kcservices.HandlerHTTP,
					kcservices.HandlerRedirect,
				},
			},
		}

	} else {
		for _, port := range opts.Ports {
			var service kcservices.CreateRequestService

			if strings.ContainsRune(port, '/') {
				split := strings.Split(port, "/")
				if len(split) != 2 {
					return nil, nil, fmt.Errorf("malformed port expected format EXTERNAL:INTERNAL[/HANDLER[,HANDLER...]]")
				}

				for _, handler := range strings.Split(split[1], "+") {
					h := kcservices.Handler(handler)
					if !slices.Contains(kcservices.Handlers(), h) {
						return nil, nil, fmt.Errorf("unknown handler: %s (choice of %v)", handler, kcservices.Handlers())
					}

					service.Handlers = append(service.Handlers, h)
				}

				port = split[0]
			}

			if strings.ContainsRune(port, ':') {
				ports := strings.Split(port, ":")
				if len(ports) != 2 {
					return nil, nil, fmt.Errorf("invalid --port value expected --port EXTERNAL:INTERNAL")
				}

				service.Port, err = strconv.Atoi(ports[0])
				if err != nil {
					return nil, nil, fmt.Errorf("invalid internal port: %w", err)
				}

				dstPort, err := strconv.Atoi(ports[1])
				if err != nil {
					return nil, nil, fmt.Errorf("invalid external port: %w", err)
				}
				service.DestinationPort = &dstPort
			} else {
				port, err := strconv.Atoi(port)
				if err != nil {
					return nil, nil, fmt.Errorf("could not parse port number: %w", err)
				}

				service.Port = port
				service.DestinationPort = &port
			}

			services = append(services, service)
		}
	}

	if len(opts.ServiceGroupNameOrUUID) == 0 {
		if len(services) > 0 {
			req.ServiceGroup = &kcinstances.CreateRequestServiceGroup{
				Services: services,
			}
		}
		if opts.SubDomain != "" {
			dnsName := strings.TrimSuffix(opts.SubDomain, ".")
			if req.ServiceGroup == nil {
				req.ServiceGroup = &kcinstances.CreateRequestServiceGroup{
					Domains: []kcservices.CreateRequestDomain{{
						Name: dnsName,
					}},
					Services: services,
				}
			} else {
				req.ServiceGroup.Domains = []kcservices.CreateRequestDomain{{
					Name: dnsName,
				}}
			}
		} else if opts.FQDN != "" {
			if !strings.HasSuffix(".", opts.FQDN) {
				opts.FQDN += "."
			}

			if req.ServiceGroup == nil {
				req.ServiceGroup = &kcinstances.CreateRequestServiceGroup{
					Domains: []kcservices.CreateRequestDomain{{
						Name: opts.FQDN,
					}},
					Services: services,
				}
			} else {
				req.ServiceGroup.Domains = []kcservices.CreateRequestDomain{{
					Name: opts.FQDN,
				}}
			}
		}
	}

	if len(opts.Env) > 0 && req.Env == nil {
		req.Env = make(map[string]string, len(opts.Env))
	}
	for _, env := range opts.Env {
		if strings.ContainsRune(env, '=') {
			split := strings.SplitN(env, "=", 2)
			req.Env[split[0]] = split[1]
		} else {
			req.Env[env] = os.Getenv(env)
		}
	}

	newInstanceResp, err := opts.Client.Instances().WithMetro(opts.Metro).Create(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	newInstance, err := newInstanceResp.FirstOrErr()
	if err != nil {
		return nil, nil, err
	}

	instanceResp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, newInstance.UUID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting details of instance %s: %w", newInstance.UUID, err)
	}
	instance, err := instanceResp.FirstOrErr()
	if err != nil {
		return nil, nil, fmt.Errorf("getting details of instance %s: %w", newInstance.UUID, err)
	}

	if sg := instance.ServiceGroup; sg != nil && sg.UUID != "" {
		serviceGroupResp, err := opts.Client.Services().WithMetro(opts.Metro).Get(ctx, sg.UUID)
		if err != nil {
			return nil, nil, fmt.Errorf("getting details of service %s: %w", sg.UUID, err)
		}
		if serviceGroup, err = serviceGroupResp.FirstOrErr(); err != nil {
			return nil, nil, fmt.Errorf("getting details of service %s: %w", sg.UUID, err)
		}
	}

	return instanceResp, serviceGroup, nil
}

// Rollout an instance in a service group.
func Rollout(ctx context.Context, opts *CreateOptions, newInstance *kcinstances.GetResponseItem, newServiceGroup *kcservices.GetResponseItem) error {
	oneMinute := int(time.Minute.Milliseconds())

	_, err := opts.Client.Instances().WithMetro(opts.Metro).Wait(ctx, kcinstances.StateRunning, oneMinute, newInstance.UUID)
	if err != nil {
		return fmt.Errorf("could not wait for the first new instance to start: %w", err)
	}

	if newServiceGroup == nil {
		return fmt.Errorf("empty service group")
	}

	// First stop one instance which is not the new one
	for i, instance := range newServiceGroup.Instances {
		if instance.UUID == newInstance.UUID {
			continue
		}

		log.G(ctx).
			WithField("instance", instance.Name).
			WithField("service group", newServiceGroup.Name).
			Info("draining old instance")

		if _, err = opts.Client.Instances().WithMetro(opts.Metro).Stop(ctx, oneMinute, false, instance.UUID); err != nil {
			return fmt.Errorf("could not stop the old instance: %w", err)
		}

		if _, err = opts.Client.Instances().WithMetro(opts.Metro).Wait(ctx, kcinstances.StateStopped, oneMinute, instance.UUID); err != nil {
			return fmt.Errorf("could not wait for the old instance to stop: %w", err)
		}

		log.G(ctx).
			WithField("instance", instance).
			WithField("service group", newServiceGroup.Name).
			Info("drained old instance")

		newServiceGroup.Instances = append(newServiceGroup.Instances[:i], newServiceGroup.Instances[i+1:]...)
		break
	}

	for _, instance := range newServiceGroup.Instances {
		if instance.UUID == newInstance.UUID {
			continue
		}

		log.G(ctx).
			WithField("service group", newServiceGroup.Name).
			Info("starting new instance")

		// Create the rest of the instances and wait max 10s for them to start
		timeout := int(time.Second.Milliseconds() * 10)
		autoStart := true
		req := kcinstances.CreateRequest{
			Image: newInstance.Image,
			Args:  newInstance.Args,
			Env:   newInstance.Env,
			ServiceGroup: &kcinstances.CreateRequestServiceGroup{
				UUID: &newServiceGroup.UUID,
			},
			Autostart:     &autoStart,
			WaitTimeoutMs: &timeout,
		}

		if opts.Memory > 0 {
			req.MemoryMB = &opts.Memory
		}

		if _, err := opts.Client.Instances().WithMetro(opts.Metro).Create(ctx, req); err != nil {
			return fmt.Errorf("could not create a new instance: %w", err)
		}

		log.G(ctx).
			WithField("instance", instance).
			WithField("service group", newServiceGroup.Name).
			Info("draining old instance")

		if _, err := opts.Client.Instances().WithMetro(opts.Metro).Stop(ctx, oneMinute, false, instance.UUID); err != nil {
			return fmt.Errorf("could not stop the old instance: %w", err)
		}

		if _, err = opts.Client.Instances().WithMetro(opts.Metro).Wait(ctx, kcinstances.StateStopped, oneMinute, instance.UUID); err != nil {
			return fmt.Errorf("could not wait for the old instance to stop: %w", err)
		}

		log.G(ctx).
			WithField("instance", instance).
			WithField("service group", newServiceGroup.Name).
			Info("drained old instance")
	}

	return nil
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
	if domain != "" && opts.FQDN != "" {
		return fmt.Errorf("cannot use --domain and --fqdn together")
	} else if domain != "" && opts.FQDN == "" {
		opts.FQDN = domain
	}

	if opts.Rollout && opts.ServiceGroupNameOrUUID == "" {
		return errors.New("cannot use --rollout without a --service-group")
	}

	if opts.Rollout && opts.Replicas > 0 {
		return errors.New("cannot use --rollout with --replicas")
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	log.G(cmd.Context()).WithField("metro", opts.Metro).Debug("using")
	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	opts.Image = args[0]

	instanceResp, serviceGroup, err := Create(ctx, opts, args[1:]...)
	if err != nil {
		return err
	}
	instance, err := instanceResp.FirstOrErr()
	if err != nil {
		return err
	}

	if opts.Rollout {
		if len(serviceGroup.Instances) == 1 {
			log.G(ctx).Warn("cannot perform a rolling update on no instances")
			return nil
		}

		paramodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(false),
				processtree.WithRenderer(
					log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
				),
				processtree.WithFailFast(true),
				processtree.WithHideOnSuccess(false),
				processtree.WithTimeout(60),
			},
			processtree.NewProcessTreeItem(
				"updating "+fmt.Sprintf("%d", len(serviceGroup.Instances)-1)+" instances of "+serviceGroup.Name,
				"",
				func(ctx context.Context) error {
					return Rollout(ctx, opts, instance, serviceGroup)
				},
			),
		)
		if err != nil {
			return nil
		}

		err = paramodel.Start()
		if err != nil {
			return fmt.Errorf("could not start the process tree: %w", err)
		}
	}

	if opts.Output != "table" && opts.Output != "full" {
		return utils.PrintInstances(ctx, opts.Output, instanceResp)
	}
	utils.PrettyPrintInstance(ctx, instance, serviceGroup, opts.Start)

	return nil
}

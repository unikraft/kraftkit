// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import (
	"context"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/resource"

	cloud "sdk.kraft.cloud"
	ukcclient "sdk.kraft.cloud/client"
	ukcimages "sdk.kraft.cloud/images"
	ukcinstances "sdk.kraft.cloud/instances"
	ukcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/tui/selection"
)

type CreateOptions struct {
	Auth                *config.AuthConfig              `noattribute:"true"`
	Client              cloud.KraftCloud                `noattribute:"true"`
	Certificate         []string                        `local:"true" long:"certificate" short:"c" usage:"Set the certificates to use for the service"`
	Env                 []string                        `local:"true" long:"env" short:"e" usage:"Environmental variables"`
	Features            []string                        `local:"true" long:"feature" short:"f" usage:"List of features to enable"`
	Domain              []string                        `local:"true" long:"domain" short:"d" usage:"The domain names to use for the service"`
	Image               string                          `noattribute:"true"`
	Entrypoint          types.ShellCommand              `local:"true" long:"entrypoint" usage:"Set the entrypoint for the instance"`
	Memory              string                          `local:"true" long:"memory" short:"M" usage:"Specify the amount of memory to allocate (MiB increments)"`
	Metro               string                          `noattribute:"true"`
	Name                string                          `local:"true" long:"name" short:"n" usage:"Specify the name of the instance"`
	Output              string                          `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"list"`
	Ports               []string                        `local:"true" long:"port" short:"p" usage:"Specify the port mapping between external to internal"`
	RestartPolicy       *ukcinstances.RestartPolicy     `noattribute:"true"`
	Replicas            uint                            `local:"true" long:"replicas" short:"R" usage:"Number of replicas of the instance" default:"0"`
	Rollout             *RolloutStrategy                `noattribute:"true"`
	RolloutQualifier    *RolloutQualifier               `noattribute:"true"`
	RolloutWait         time.Duration                   `local:"true" long:"rollout-wait" usage:"Time to wait before performing rolling out action (ms/s/m/h)" default:"10s"`
	ServiceNameOrUUID   string                          `local:"true" long:"service" short:"g" usage:"Attach this instance to an existing service"`
	Start               bool                            `local:"true" long:"start" short:"S" usage:"Immediately start the instance after creation"`
	ScaleToZero         *ukcinstances.ScaleToZeroPolicy `noattribute:"true"`
	ScaleToZeroStateful *bool                           `local:"true" long:"scale-to-zero-stateful" usage:"Save state when scaling to zero"`
	ScaleToZeroCooldown time.Duration                   `local:"true" long:"scale-to-zero-cooldown" usage:"Cooldown period before scaling to zero (ms/s/m/h)"`
	SubDomain           []string                        `local:"true" long:"subdomain" short:"s" usage:"Set the subdomains to use when creating the service"`
	Token               string                          `noattribute:"true"`
	Vcpus               uint                            `local:"true" long:"vcpus" short:"V" usage:"Specify the number of vCPUs to allocate"`
	Volumes             []string                        `local:"true" long:"volume" short:"v" usage:"List of volumes to attach instance to"`
	WaitForImage        bool                            `local:"true" long:"wait-for-image" short:"w" usage:"Wait for the image to be available before creating the instance"`
	WaitForImageTimeout time.Duration                   `local:"true" long:"wait-for-image-timeout" usage:"Time to wait before timing out when waiting for image (ms/s/m/h)" default:"60s"`

	Services []ukcservices.CreateRequestService `noattribute:"true"`
}

// Create a UnikraftCloud instance.
func Create(ctx context.Context, opts *CreateOptions, args ...string) (*ukcclient.ServiceResponse[ukcinstances.GetResponseItem], *ukcclient.ServiceResponse[ukcservices.GetResponseItem], error) {
	var err error

	if opts == nil {
		opts = &CreateOptions{}
	}

	if len(opts.Domain) > 0 && len(opts.Certificate) > 0 && len(opts.Domain) != len(opts.Certificate) {
		return nil, nil, fmt.Errorf("number of certificates does not match number of domains")
	}

	if opts.Auth == nil {
		opts.Auth, err = config.GetUnikraftCloudAuthConfig(ctx, opts.Token)
		if err != nil {
			return nil, nil, fmt.Errorf("could not retrieve credentials: %w", err)
		}
	}
	if opts.Client == nil {
		opts.Client = cloud.NewClient(
			cloud.WithToken(config.GetUnikraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	if opts.RestartPolicy == nil {
		opts.RestartPolicy = ptr(ukcinstances.RestartPolicyNever)
	}

	if opts.Rollout == nil {
		opts.Rollout = ptr(StrategyPrompt)
	}

	if opts.RolloutQualifier == nil {
		opts.RolloutQualifier = ptr(RolloutQualifierImageName)
	}

	// Check if the user tries to use a service and a rollout strategy is
	// set to prompt so that we can use this information later.  We do this very
	// early on such that we can fail-fast in case prompting is not possible (e.g.
	// in non-TTY environments).
	if opts.ServiceNameOrUUID != "" && *opts.Rollout == StrategyPrompt {
		if config.G[config.KraftKit](ctx).NoPrompt {
			return nil, nil, fmt.Errorf("prompting disabled")
		}
	}

	if !strings.Contains(opts.Image, ":") {
		opts.Image += ":latest"
	}

	// Sanitize image name
	opts.Image = strings.TrimPrefix(opts.Image, "index.unikraft.io/")
	opts.Image = strings.TrimPrefix(opts.Image, "official/")

	// Replace all slashes in the name with dashes.
	opts.Name = strings.ReplaceAll(opts.Name, "/", "-")

	// Keep a reference of the image that we are going to use for the instance.
	var image *ukcimages.GetResponseItem

	// Check if the image exists before creating the instance
	if opts.WaitForImage {
		paramodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(false),
				processtree.WithRenderer(
					log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
				),
				processtree.WithFailFast(true),
				processtree.WithHideOnSuccess(true),
				processtree.WithTimeout(opts.WaitForImageTimeout),
			},
			processtree.NewProcessTreeItem(
				"propagating",
				"",
				func(ctx context.Context) error {
					for {
						imageResp, err := opts.Client.Images().WithMetro(opts.Metro).Get(ctx, opts.Image)
						if err != nil {
							continue
						}

						image, err = imageResp.FirstOrErr()
						if err != nil || image == nil {
							continue
						}

						return nil
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
	} else {
		imageResp, err := opts.Client.Images().WithMetro(opts.Metro).Get(ctx, opts.Image)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get image: %w", err)
		}

		image, err = imageResp.FirstOrErr()
		if err != nil {
			return nil, nil, fmt.Errorf("could not get image: %w", err)
		}

		if image == nil {
			return nil, nil, fmt.Errorf("no image with name: %s", opts.Image)
		}
	}

	var features []ukcinstances.Feature

	for _, feature := range opts.Features {
		formattedFeature := ukcinstances.Feature(feature)
		if !slices.Contains(features, formattedFeature) {
			features = append(features, formattedFeature)
		}
	}

	req := ukcinstances.CreateRequest{
		Autostart:     &opts.Start,
		Features:      features,
		Image:         opts.Image,
		RestartPolicy: opts.RestartPolicy,
	}
	if opts.Vcpus > 0 {
		req.Vcpus = ptr(int(opts.Vcpus))
	}
	if opts.Name != "" {
		req.Name = &opts.Name
	}
	if opts.Entrypoint.IsZero() {
		req.Args = []string{image.Args}
	} else {
		req.Args = opts.Entrypoint
	}
	if len(args) > 0 {
		req.Args = append(req.Args, args...)
	}
	if opts.Memory != "" {
		if _, err := strconv.ParseUint(opts.Memory, 10, 64); err == nil {
			opts.Memory = fmt.Sprintf("%sMi", opts.Memory)
		}

		qty, err := resource.ParseQuantity(opts.Memory)
		if err != nil {
			return nil, nil, fmt.Errorf("could not parse memory quantity: %w", err)
		}

		if qty.Value() < 1024*1024 {
			return nil, nil, fmt.Errorf("memory must be at least 1Mi")
		}

		// Convert to MiB
		req.MemoryMB = ptr(int(qty.Value() / (1024 * 1024)))
	} else {
		// Set the default memory to the size of the image rounded to the nearest
		// power of 2 with an arbitrary 10% buffer.  Only set the value if it is
		// greater than 128 MiB.
		mem := int(math.Round(math.Pow(2, math.Log2(float64(image.SizeInBytes/1024/1024)*1.1))))
		if mem > 128 {
			req.MemoryMB = &mem
		}
	}
	if opts.Replicas > 0 {
		req.Replicas = ptr(int(opts.Replicas))
	}

	if len(opts.Services) == 0 && opts.ServiceNameOrUUID == "" && len(opts.Ports) == 0 {
		log.G(ctx).Info("no ports or service specified, disabling scale to zero")
		opts.ScaleToZeroCooldown = 0
		opts.ScaleToZeroStateful = nil
		off := ukcinstances.ScaleToZeroPolicyOff
		opts.ScaleToZero = &off
	}

	if opts.ScaleToZeroCooldown != 0 || opts.ScaleToZeroStateful != nil || opts.ScaleToZero != nil {
		req.ScaleToZero = &ukcinstances.ScaleToZero{}
	}

	if opts.ScaleToZeroCooldown != 0 {
		req.ScaleToZero.CooldownTimeMs = ptr(int(opts.ScaleToZeroCooldown.Milliseconds()))
	}

	if opts.ScaleToZeroStateful != nil {
		req.ScaleToZero.Stateful = opts.ScaleToZeroStateful
	}

	if opts.ScaleToZero != nil {
		req.ScaleToZero.Policy = opts.ScaleToZero
	}

	for _, vol := range opts.Volumes {
		split := strings.Split(vol, ":")
		if len(split) < 2 || len(split) > 3 {
			return nil, nil, fmt.Errorf("invalid syntax for -v|--volume: expected VOLUME:PATH[:ro]")
		}
		volume := ukcinstances.CreateRequestVolume{
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

	var service *ukcservices.GetResponseItem
	var qualifiedInstancesToRolloutOver []ukcinstances.GetResponseItem

	// Since an existing service has been provided, we should now
	// preemptively look up information about it.  Based on whether there are
	// active instances inside of this service, we can then decide how to
	// proceed with the deployment (aka rollout strategies).
	if len(opts.ServiceNameOrUUID) > 0 {
		log.G(ctx).
			WithField("service", opts.ServiceNameOrUUID).
			Trace("finding")

		serviceResp, err := opts.Client.Services().WithMetro(opts.Metro).Get(ctx, opts.ServiceNameOrUUID)
		if err != nil {
			return nil, nil, fmt.Errorf("could not use service %s: %w", opts.ServiceNameOrUUID, err)
		}

		service, err = serviceResp.FirstOrErr()
		if err != nil {
			return nil, nil, fmt.Errorf("could not use service %s: %w", opts.ServiceNameOrUUID, err)
		}

		log.G(ctx).
			WithField("uuid", service.UUID).
			Debug("using service")

		// Save the UUID of the service to be used in the create request
		// later.
		req.ServiceGroup = &ukcinstances.CreateRequestServiceGroup{
			UUID: &service.UUID,
		}

		if opts.Start {
			// Find out if there are any existing instances in this service.
			allInstanceUUIDs := make([]string, len(service.Instances))
			for i, instance := range service.Instances {
				allInstanceUUIDs[i] = instance.UUID
			}

			var instances []ukcinstances.GetResponseItem
			if len(allInstanceUUIDs) > 0 {
				log.G(ctx).
					WithField("service", opts.ServiceNameOrUUID).
					Trace("getting instances in")

				instancesResp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, allInstanceUUIDs...)
				if err != nil {
					return nil, nil, fmt.Errorf("could not get instances of service '%s': %w", opts.ServiceNameOrUUID, err)
				}

				instances, err = instancesResp.AllOrErr()
				if err != nil {
					return nil, nil, fmt.Errorf("could not get instances of service '%s': %w", opts.ServiceNameOrUUID, err)
				}
			} else {
				log.G(ctx).
					WithField("service", opts.ServiceNameOrUUID).
					Trace("no existing instances in service")
			}

			switch *opts.RolloutQualifier {
			case RolloutQualifierImageName:
				imageBase := opts.Image
				if strings.Contains(opts.Image, "@") {
					imageBase, _, _ = strings.Cut(opts.Image, "@")
				} else if strings.Contains(opts.Image, ":") {
					imageBase, _, _ = strings.Cut(opts.Image, ":")
				}

				for _, instance := range instances {
					instImageBase, _, _ := strings.Cut(instance.Image, "@")
					if instImageBase == imageBase {
						qualifiedInstancesToRolloutOver = append(qualifiedInstancesToRolloutOver, instance)
					}
				}

			case RolloutQualifierInstanceName:
				for _, instance := range instances {
					if instance.Name == opts.Name {
						qualifiedInstancesToRolloutOver = append(qualifiedInstancesToRolloutOver, instance)
					}
				}

			case RolloutQualifierAll:
				qualifiedInstancesToRolloutOver = instances

			default: // case RolloutQualifierNone:
				// No-op
			}

			if len(qualifiedInstancesToRolloutOver) > 0 && *opts.Rollout == StrategyPrompt {
				strategy, err := selection.Select(
					fmt.Sprintf("deployment already exists: what would you like to do with the %d existing instance(s)?", len(qualifiedInstancesToRolloutOver)),
					RolloutStrategies()...,
				)
				if err != nil {
					return nil, nil, err
				}

				log.G(ctx).Infof("use --rollout=%s to skip this prompt in the future", strategy.String())

				opts.Rollout = strategy
			}

			// Return early if the rollout strategy is set to exit on conflict and there
			// are existing instances in the service.
			if *opts.Rollout == RolloutStrategyAbort {
				return nil, nil, fmt.Errorf("deployment already exists and merge strategy set to exit on conflict")
			}
		}
	}

	// TODO(nderjung): This should eventually be possible, when the UnikraftCloud API
	// supports updating service.
	if opts.ServiceNameOrUUID != "" && len(opts.Ports) > 0 {
		return nil, nil, fmt.Errorf("cannot use existing --service|-g and define new --port|-p")
	}

	// TODO(nderjung): This should eventually be possible, when the UnikraftCloud API
	// supports updating service.
	if opts.ServiceNameOrUUID != "" && len(opts.Domain) > 0 {
		return nil, nil, fmt.Errorf("cannot use existing --service|-g and define new --domain|-d")
	}

	// TODO(nderjung): This should eventually be possible, when the UnikraftCloud API
	// supports updating service groups.
	if opts.ServiceNameOrUUID != "" && len(opts.Certificate) > 0 {
		return nil, nil, fmt.Errorf("cannot use existing --service-group|-g and define new --certificate|-c")
	}

	// TODO(nderjung): This should eventually be possible, when the UnikraftCloud API
	// supports updating service groups.
	if opts.ServiceNameOrUUID != "" && len(opts.SubDomain) > 0 {
		return nil, nil, fmt.Errorf("cannot use existing --service-group|-g and define new --subdomain|-s")
	}

	if len(opts.Ports) == 1 && strings.HasPrefix(opts.Ports[0], "443:") && strings.Count(opts.Ports[0], "/") == 0 {
		split := strings.Split(opts.Ports[0], ":")
		if len(split) != 2 {
			return nil, nil, fmt.Errorf("malformed port expected format EXTERNAL:INTERNAL[/HANDLER]")
		}

		destPort, err := strconv.Atoi(split[1])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid external port: %w", err)
		}

		port443 := 443
		opts.Services = []ukcservices.CreateRequestService{
			{
				Port:            443,
				DestinationPort: &destPort,
				Handlers: []ukcservices.Handler{
					ukcservices.HandlerHTTP,
					ukcservices.HandlerTLS,
				},
			},
			{
				Port:            80,
				DestinationPort: &port443,
				Handlers: []ukcservices.Handler{
					ukcservices.HandlerHTTP,
					ukcservices.HandlerRedirect,
				},
			},
		}

	} else {
		for _, port := range opts.Ports {
			var service ukcservices.CreateRequestService

			if strings.ContainsRune(port, '/') {
				split := strings.Split(port, "/")
				if len(split) != 2 {
					return nil, nil, fmt.Errorf("malformed port expected format EXTERNAL:INTERNAL[/HANDLER]")
				}

				for _, handler := range strings.Split(split[1], "+") {
					h := ukcservices.Handler(handler)
					if !slices.Contains(ukcservices.Handlers(), h) {
						return nil, nil, fmt.Errorf("unknown handler: %s (choice of %v)", handler, ukcservices.Handlers())
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

			opts.Services = append(opts.Services, service)
		}
	}

	if len(opts.ServiceNameOrUUID) == 0 {
		if len(opts.Services) > 0 {
			req.ServiceGroup = &ukcinstances.CreateRequestServiceGroup{
				Services: opts.Services,
			}
		}
		if len(opts.SubDomain) > 0 {
			if req.ServiceGroup == nil {
				req.ServiceGroup = &ukcinstances.CreateRequestServiceGroup{
					Domains:  []ukcservices.CreateRequestDomain{},
					Services: opts.Services,
				}
			} else {
				if req.ServiceGroup.Domains == nil {
					req.ServiceGroup.Domains = []ukcservices.CreateRequestDomain{}
				}
			}
			for _, subDomain := range opts.SubDomain {
				if subDomain == "" {
					continue
				}

				dnsName := strings.TrimSuffix(subDomain, ".")

				req.ServiceGroup.Domains = append(req.ServiceGroup.Domains, ukcservices.CreateRequestDomain{
					Name: dnsName,
				})
			}
		} else if len(opts.Domain) > 0 {
			if req.ServiceGroup == nil {
				req.ServiceGroup = &ukcinstances.CreateRequestServiceGroup{
					Domains:  []ukcservices.CreateRequestDomain{},
					Services: opts.Services,
				}
			} else {
				if req.ServiceGroup.Domains == nil {
					req.ServiceGroup.Domains = []ukcservices.CreateRequestDomain{}
				}
			}

			for i, fqdn := range opts.Domain {
				if fqdn == "" {
					continue
				}

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

				req.ServiceGroup.Domains = append(req.ServiceGroup.Domains, domainCreate)
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

	// Handle the rollout only after the new instance has been created.
	// UnikraftCloud's service load balancer will temporarily handle blue-green
	// deployments.
	if opts.Start && len(qualifiedInstancesToRolloutOver) > 0 {
		paramodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(false),
				processtree.WithRenderer(
					log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
				),
				processtree.WithFailFast(true),
				processtree.WithHideOnSuccess(true),
			},
			processtree.NewProcessTreeItem(
				"waiting for new instance to start before performing rollout action",
				"",
				func(ctx context.Context) error {
					_, err := opts.Client.Instances().WithMetro(opts.Metro).Wait(ctx, ukcinstances.StateRunning, int(opts.RolloutWait.Milliseconds()), newInstance.UUID)
					return err
				},
			),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("could not start wait process: %w", err)
		}

		if err = paramodel.Start(); err != nil {
			log.G(ctx).
				WithError(err).
				Error("aborting rollout: could not wait for new instance to start")
		} else {
			var batch []string
			for _, instance := range qualifiedInstancesToRolloutOver {
				log.G(ctx).
					WithField("instance", instance.UUID).
					Debug("qualified")
				batch = append(batch, instance.UUID)
			}

			switch *opts.Rollout {
			case RolloutStrategyStop:
				log.G(ctx).Infof("stopping %d existing instance(s)", len(qualifiedInstancesToRolloutOver))
				if _, err = opts.Client.Instances().WithMetro(opts.Metro).Stop(ctx, 60, false, batch...); err != nil {
					return nil, nil, fmt.Errorf("could not stop instance(s): %w", err)
				}

			case RolloutStrategyRemove:
				log.G(ctx).Infof("removing %d existing instance(s)", len(qualifiedInstancesToRolloutOver))
				if _, err = opts.Client.Instances().WithMetro(opts.Metro).Delete(ctx, batch...); err != nil {
					return nil, nil, fmt.Errorf("could not delete instance(s): %w", err)
				}
			}
		}
	}

	instanceResp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, newInstance.UUID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting details of instance %s: %w", newInstance.UUID, err)
	}
	instance, err := instanceResp.FirstOrErr()
	if err != nil {
		return nil, nil, fmt.Errorf("getting details of instance %s: %w", newInstance.UUID, err)
	}

	var serviceResp *ukcclient.ServiceResponse[ukcservices.GetResponseItem]

	if sg := instance.ServiceGroup; sg != nil && sg.UUID != "" {
		serviceResp, err = opts.Client.Services().WithMetro(opts.Metro).Get(ctx, sg.UUID)
		if err != nil {
			return nil, nil, fmt.Errorf("getting details of service %s: %w", sg.UUID, err)
		}
	}

	return instanceResp, serviceResp, nil
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
			Create an instance on Unikraft Cloud from an image.
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-instance",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[RolloutStrategy](
			append(RolloutStrategies(), StrategyPrompt),
			StrategyPrompt,
		),
		"rollout",
		"Set the rollout strategy for an instance which has been previously run in the provided service",
	)

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[RolloutQualifier](
			RolloutQualifiers(),
			RolloutQualifierImageName,
		),
		"rollout-qualifier",
		"Set the rollout qualifier used to determine which instances should be affected by the strategy in the supplied service",
	)

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[ukcinstances.RestartPolicy](
			ukcinstances.RestartPolicies(),
			ukcinstances.RestartPolicyNever,
		),
		"restart",
		"Set the restart policy for the instance (never/always/on-failure)",
	)

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[ukcinstances.ScaleToZeroPolicy](
			ukcinstances.ScaleToZeroPolicies(),
			ukcinstances.ScaleToZeroPolicyOff,
		),
		"scale-to-zero",
		"Scale to zero policy of the instance (on/off/idle)",
	)

	return cmd
}

func (opts *CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	if opts.ScaleToZeroCooldown != 0 && opts.ScaleToZeroCooldown < time.Millisecond {
		return fmt.Errorf("scale-to-zero-cooldown needs to be at least 1ms: %s", opts.ScaleToZeroCooldown)
	}

	opts.RestartPolicy = ptr(ukcinstances.RestartPolicy(cmd.Flag("restart").Value.String()))
	opts.Rollout = ptr(RolloutStrategy(cmd.Flag("rollout").Value.String()))
	opts.RolloutQualifier = ptr(RolloutQualifier(cmd.Flag("rollout-qualifier").Value.String()))

	if cmd.Flag("scale-to-zero").Changed {
		s20v := ukcinstances.ScaleToZeroPolicy(cmd.Flag("scale-to-zero").Value.String())
		opts.ScaleToZero = &s20v
	}

	if cmd.Flag("scale-to-zero-stateful").Changed {
		statefulFlag, err := strconv.ParseBool(cmd.Flag("scale-to-zero-stateful").Value.String())
		if err != nil {
			return fmt.Errorf("could not parse scale-to-zero-stateful: %w", err)
		}

		opts.ScaleToZeroStateful = &statefulFlag
	}

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	return nil
}

func (opts *CreateOptions) Run(ctx context.Context, args []string) error {
	opts.Image = args[0]

	instResp, svcResp, err := Create(ctx, opts, args[1:]...)
	if err != nil {
		return err
	}

	insts, err := instResp.AllOrErr()
	if err != nil {
		return err
	}

	if len(insts) > 1 || opts.Output == "table" || opts.Output == "json" {
		return utils.PrintInstances(ctx, opts.Output, *instResp)
	}

	// No need to check for error, we check if-nil inside PrettyPrintInstance.
	svc, _ := svcResp.FirstOrErr()
	utils.PrettyPrintInstance(ctx, opts.Metro, insts[0], svc, opts.Start)

	return nil
}

func ptr[T comparable](v T) *T { return &v }

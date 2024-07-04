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

	kraftcloud "sdk.kraft.cloud"
	kcclient "sdk.kraft.cloud/client"
	kcimages "sdk.kraft.cloud/images"
	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/tui/selection"
)

type CreateOptions struct {
	Auth                   *config.AuthConfig        `noattribute:"true"`
	Client                 kraftcloud.KraftCloud     `noattribute:"true"`
	Env                    []string                  `local:"true" long:"env" short:"e" usage:"Environmental variables"`
	Features               []string                  `local:"true" long:"feature" short:"f" usage:"List of features to enable"`
	Domain                 []string                  `local:"true" long:"domain" short:"d" usage:"The domain names to use for the service"`
	Image                  string                    `noattribute:"true"`
	Entrypoint             types.ShellCommand        `local:"true" long:"entrypoint" usage:"Set the entrypoint for the instance"`
	Memory                 string                    `local:"true" long:"memory" short:"M" usage:"Specify the amount of memory to allocate (MiB increments)"`
	Metro                  string                    `noattribute:"true"`
	Name                   string                    `local:"true" long:"name" short:"n" usage:"Specify the name of the instance"`
	Output                 string                    `local:"true" long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	Ports                  []string                  `local:"true" long:"port" short:"p" usage:"Specify the port mapping between external to internal"`
	RestartPolicy          kcinstances.RestartPolicy `noattribute:"true"`
	Replicas               uint                      `local:"true" long:"replicas" short:"R" usage:"Number of replicas of the instance" default:"0"`
	Rollout                RolloutStrategy           `noattribute:"true"`
	RolloutQualifier       RolloutQualifier          `noattribute:"true"`
	RolloutWait            time.Duration             `local:"true" long:"rollout-wait" usage:"Time to wait before performing rolling out action" default:"10s"`
	ServiceGroupNameOrUUID string                    `local:"true" long:"service-group" short:"g" usage:"Attach this instance to an existing service group"`
	Start                  bool                      `local:"true" long:"start" short:"S" usage:"Immediately start the instance after creation"`
	ScaleToZero            bool                      `local:"true" long:"scale-to-zero" short:"0" usage:"Scale the instance to zero after deployment"`
	SubDomain              []string                  `local:"true" long:"subdomain" short:"s" usage:"Set the subdomains to use when creating the service"`
	Token                  string                    `noattribute:"true"`
	Volumes                []string                  `local:"true" long:"volumes" short:"v" usage:"List of volumes to attach instance to"`
	WaitForImage           bool                      `local:"true" long:"wait-for-image" short:"w" usage:"Wait for the image to be available before creating the instance"`
	WaitForImageTimeout    time.Duration             `local:"true" long:"wait-for-image-timeout" usage:"Time to wait before timing out when waiting for image" default:"60s"`

	Services []kcservices.CreateRequestService `noattribute:"true"`
}

// Create a KraftCloud instance.
func Create(ctx context.Context, opts *CreateOptions, args ...string) (*kcclient.ServiceResponse[kcinstances.GetResponseItem], *kcclient.ServiceResponse[kcservices.GetResponseItem], error) {
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

	// Check if the user tries to use a service group and a rollout strategy is
	// set to prompt so that we can use this information later.  We do this very
	// early on such that we can fail-fast in case prompting is not possible (e.g.
	// in non-TTY environments).
	if opts.ServiceGroupNameOrUUID != "" && opts.Rollout == StrategyPrompt {
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
	var image *kcimages.GetResponseItem

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
				"waiting for the image to be available",
				opts.Image,
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
		Autostart:     &opts.Start,
		Features:      features,
		Image:         opts.Image,
		RestartPolicy: &opts.RestartPolicy,
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
	var qualifiedInstancesToRolloutOver []kcinstances.GetResponseItem

	// Since an existing service group has been provided, we should now
	// preemptively look up information about it.  Based on whether there are
	// active instances inside of this service group, we can then decide how to
	// proceed with the deployment (aka rollout strategies).
	if len(opts.ServiceGroupNameOrUUID) > 0 {
		log.G(ctx).
			WithField("group", opts.ServiceGroupNameOrUUID).
			Trace("looking up service")

		groupResp, err := opts.Client.Services().WithMetro(opts.Metro).Get(ctx, opts.ServiceGroupNameOrUUID)
		if err != nil {
			return nil, nil, fmt.Errorf("could not use service %s: %w", opts.ServiceGroupNameOrUUID, err)
		}

		serviceGroup, err = groupResp.FirstOrErr()
		if err != nil {
			return nil, nil, fmt.Errorf("could not use service %s: %w", opts.ServiceGroupNameOrUUID, err)
		}

		log.G(ctx).
			WithField("uuid", serviceGroup.UUID).
			Debug("using service group")

		// Save the UUID of the service group to be used in the create request
		// later.
		req.ServiceGroup = &kcinstances.CreateRequestServiceGroup{
			UUID: &serviceGroup.UUID,
		}

		// Find out if there are any existing instances in this service group.
		allInstanceUUIDs := make([]string, len(serviceGroup.Instances))
		for i, instance := range serviceGroup.Instances {
			allInstanceUUIDs[i] = instance.UUID
		}

		var instances []kcinstances.GetResponseItem
		if len(allInstanceUUIDs) > 0 {
			log.G(ctx).
				WithField("group", opts.ServiceGroupNameOrUUID).
				Trace("getting instances in service")

			instancesResp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, allInstanceUUIDs...)
			if err != nil {
				return nil, nil, fmt.Errorf("could not get instances of service group '%s': %w", opts.ServiceGroupNameOrUUID, err)
			}

			instances, err = instancesResp.AllOrErr()
			if err != nil {
				return nil, nil, fmt.Errorf("could not get instances of service group '%s': %w", opts.ServiceGroupNameOrUUID, err)
			}
		} else {
			log.G(ctx).
				WithField("group", opts.ServiceGroupNameOrUUID).
				Trace("no existing instances in service group")
		}

		switch opts.RolloutQualifier {
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

		if len(qualifiedInstancesToRolloutOver) > 0 && opts.Rollout == StrategyPrompt {
			strategy, err := selection.Select(
				fmt.Sprintf("deployment already exists: what would you like to do with the %d existing instance(s)?", len(qualifiedInstancesToRolloutOver)),
				RolloutStrategies()...,
			)
			if err != nil {
				return nil, nil, err
			}

			log.G(ctx).Infof("use --rollout=%s to skip this prompt in the future", strategy.String())

			opts.Rollout = *strategy
		}

		// Return early if the rollout strategy is set to exit on conflict and there
		// are existing instances in the service group.
		if opts.Rollout == RolloutStrategyExit {
			return nil, nil, fmt.Errorf("deployment already exists and merge strategy set to exit on conflict")
		}
	}

	// TODO(nderjung): This should eventually be possible, when the KraftCloud API
	// supports updating service groups.
	if opts.ServiceGroupNameOrUUID != "" && len(opts.Ports) > 0 {
		return nil, nil, fmt.Errorf("cannot use existing --service-group|-g and define new --port|-p")
	}

	// TODO(nderjung): This should eventually be possible, when the KraftCloud API
	// supports updating service groups.
	if opts.ServiceGroupNameOrUUID != "" && len(opts.Domain) > 0 {
		return nil, nil, fmt.Errorf("cannot use existing --service-group|-g and define new --domain|-d")
	}

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
		opts.Services = []kcservices.CreateRequestService{
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

			opts.Services = append(opts.Services, service)
		}
	}

	if len(opts.ServiceGroupNameOrUUID) == 0 {
		if len(opts.Services) > 0 {
			req.ServiceGroup = &kcinstances.CreateRequestServiceGroup{
				Services: opts.Services,
			}
		}
		if len(opts.SubDomain) > 0 {
			if req.ServiceGroup == nil {
				req.ServiceGroup = &kcinstances.CreateRequestServiceGroup{
					Domains:  []kcservices.CreateRequestDomain{},
					Services: opts.Services,
				}
			} else {
				if req.ServiceGroup.Domains == nil {
					req.ServiceGroup.Domains = []kcservices.CreateRequestDomain{}
				}
			}
			for _, subDomain := range opts.SubDomain {
				if subDomain == "" {
					continue
				}

				dnsName := strings.TrimSuffix(subDomain, ".")

				req.ServiceGroup.Domains = append(req.ServiceGroup.Domains, kcservices.CreateRequestDomain{
					Name: dnsName,
				})
			}
		} else if len(opts.Domain) > 0 {
			if req.ServiceGroup == nil {
				req.ServiceGroup = &kcinstances.CreateRequestServiceGroup{
					Domains:  []kcservices.CreateRequestDomain{},
					Services: opts.Services,
				}
			} else {
				if req.ServiceGroup.Domains == nil {
					req.ServiceGroup.Domains = []kcservices.CreateRequestDomain{}
				}
			}

			for _, fqdn := range opts.Domain {
				if fqdn == "" {
					continue
				}

				if !strings.HasSuffix(".", fqdn) {
					fqdn += "."
				}

				req.ServiceGroup.Domains = append(req.ServiceGroup.Domains, kcservices.CreateRequestDomain{
					Name: fqdn,
				})
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
	// KraftCloud's service group load balancer will temporarily handle blue-green
	// deployments.
	if len(qualifiedInstancesToRolloutOver) > 0 {
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
					_, err := opts.Client.Instances().WithMetro(opts.Metro).Wait(ctx, kcinstances.StateRunning, int(opts.RolloutWait.Milliseconds()), newInstance.UUID)
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

			switch opts.Rollout {
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

	var serviceGroupResp *kcclient.ServiceResponse[kcservices.GetResponseItem]

	if sg := instance.ServiceGroup; sg != nil && sg.UUID != "" {
		serviceGroupResp, err = opts.Client.Services().WithMetro(opts.Metro).Get(ctx, sg.UUID)
		if err != nil {
			return nil, nil, fmt.Errorf("getting details of service %s: %w", sg.UUID, err)
		}
	}

	return instanceResp, serviceGroupResp, nil
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

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[RolloutStrategy](
			append(RolloutStrategies(), StrategyPrompt),
			StrategyPrompt,
		),
		"rollout",
		"Set the rollout strategy for an instance which has been previously run in the provided service group",
	)

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[RolloutQualifier](
			RolloutQualifiers(),
			RolloutQualifierImageName,
		),
		"rollout-qualifier",
		"Set the rollout qualifier used to determine which instances should be affected by the strategy in the supplied service group",
	)

	cmd.Flags().Var(
		cmdfactory.NewEnumFlag[kcinstances.RestartPolicy](
			kcinstances.RestartPolicies(),
			kcinstances.RestartPolicyNever,
		),
		"restart",
		"Set the restart policy for the instance (never/always/on-failure)",
	)

	return cmd
}

func (opts *CreateOptions) Pre(cmd *cobra.Command, _ []string) error {
	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	opts.RestartPolicy = kcinstances.RestartPolicy(cmd.Flag("restart").Value.String())
	opts.Rollout = RolloutStrategy(cmd.Flag("rollout").Value.String())
	opts.RolloutQualifier = RolloutQualifier(cmd.Flag("rollout-qualifier").Value.String())

	if !utils.IsValidOutputFormat(opts.Output) {
		return fmt.Errorf("invalid output format: %s", opts.Output)
	}

	log.G(cmd.Context()).WithField("metro", opts.Metro).Debug("using")
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

	if len(insts) > 1 || opts.Output == "table" || opts.Output == "list" || opts.Output == "json" {
		return utils.PrintInstances(ctx, opts.Output, *instResp)
	}

	// No need to check for error, we check if-nil inside PrettyPrintInstance.
	svc, _ := svcResp.FirstOrErr()
	utils.PrettyPrintInstance(ctx, insts[0], svc, opts.Start)

	return nil
}

func ptr[T comparable](v T) *T { return &v }

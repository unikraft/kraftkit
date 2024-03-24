// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package initialize

import (
	"context"
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcinstances "sdk.kraft.cloud/instances"
	kcautoscale "sdk.kraft.cloud/services/autoscale"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/tui/selection"
)

type InitOptions struct {
	Auth         *config.AuthConfig    `noattribute:"true"`
	Client       kraftcloud.KraftCloud `noattribute:"true"`
	CooldownTime time.Duration         `long:"cooldown-time" short:"c" usage:"The cooldown time of the config (ms/s/m/h)" default:"1000000000"`
	Master       string                `long:"master" short:"i" usage:"The UUID or Name of the master instance"`
	MaxSize      int                   `long:"max-size" short:"M" usage:"The maximum size of the configuration" default:"10"`
	Metro        string                `noattribute:"true"`
	MinSize      int                   `long:"min-size" short:"m" usage:"The minimum size of the configuration"`
	Token        string                `noattribute:"true"`
	WarmupTime   time.Duration         `long:"warmup-time" short:"w" usage:"The warmup time of the config (ms/s/m/h)" default:"1000000000"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&InitOptions{}, cobra.Command{
		Short:   "Initialize autoscale configuration for a service group",
		Use:     "initialize [FLAGS] NAME|UUID",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"init", "initialise", "i"},
		Long:    "Initialize autoscale configuration for a service group.",
		Example: heredoc.Doc(`
			# Initialize an autoscale configuration
			kraft cloud scale initialize my-service-group \
				--master my-instance-name \
				--min-size 1 \
				--max-size 10 \
				--cooldown-time 1s \
				--warmup-time 1s
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-scale",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *InitOptions) Pre(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("specify a service group name or UUID")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *InitOptions) Run(ctx context.Context, args []string) error {
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

	// Look up the configuration by name
	if !utils.IsUUID(args[0]) {
		confResp, err := opts.Client.Services().WithMetro(opts.Metro).GetByNames(ctx, args[0])
		if err != nil {
			return fmt.Errorf("could not get configuration: %w", err)
		}
		conf, err := confResp.FirstOrErr()
		if err != nil {
			return fmt.Errorf("could not get configuration: %w", err)
		}

		args[0] = conf.UUID
	}

	if opts.WarmupTime < time.Millisecond {
		return fmt.Errorf("warmup time must be at least 1ms")
	}

	if opts.CooldownTime < time.Millisecond {
		return fmt.Errorf("cooldown time must be at least 1ms")
	}

	var master kcautoscale.CreateRequestMaster

	if opts.Master == "" {
		if config.G[config.KraftKit](ctx).NoPrompt {
			return fmt.Errorf("specify an instance master UUID or name via --master")
		}

		instListResp, err := opts.Client.Instances().WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}
		instList, err := instListResp.AllOrErr()
		if err != nil {
			return fmt.Errorf("could not list instances: %w", err)
		}
		if len(instList) == 0 {
			return fmt.Errorf("no instance found in service group")
		}

		if len(instList) == 1 {
			master.UUID = &instList[0].UUID
		} else {
			var possible []stringerInstance

			uuids := make([]string, 0, len(instList))
			for _, instItem := range instList {
				uuids = append(uuids, instItem.UUID)
			}

			instancesResp, err := opts.Client.Instances().WithMetro(opts.Metro).GetByUUIDs(ctx, uuids...)
			if err != nil {
				return fmt.Errorf("getting details of %d instance(s): %w", len(instList), err)
			}
			instances, err := instancesResp.AllOrErr()
			if err != nil {
				return fmt.Errorf("getting details of %d instance(s): %w", len(instList), err)
			}

			for _, inst := range instances {
				if inst.ServiceGroup == nil {
					continue
				}
				if inst.ServiceGroup.UUID != args[0] {
					continue
				}

				possible = append(possible, stringerInstance{&inst})
			}

			result, err := selection.Select[stringerInstance](
				"select master instance",
				possible...,
			)
			if err != nil {
				return fmt.Errorf("could not select master instance: %w", err)
			}

			master.UUID = &result.UUID
		}
	} else {
		if utils.IsUUID(opts.Master) {
			master.UUID = &opts.Master
		} else {
			master.Name = &opts.Master
		}
	}

	req := kcautoscale.CreateRequest{
		UUID:   &args[0],
		Master: master,
	}
	if opts.MinSize > 0 {
		req.MinSize = &opts.MinSize
	}
	if opts.MaxSize > 0 {
		req.MaxSize = &opts.MaxSize
	}
	if opts.WarmupTime > 0 {
		warmupTimeMs := int(opts.WarmupTime.Milliseconds())
		req.WarmupTimeMs = &warmupTimeMs
	}
	if opts.CooldownTime > 0 {
		cooldownTimeMs := int(opts.CooldownTime.Milliseconds())
		req.CooldownTimeMs = &cooldownTimeMs
	}

	if _, err = opts.Client.Autoscale().WithMetro(opts.Metro).CreateConfiguration(ctx, req); err != nil {
		return fmt.Errorf("could not create configuration: %w", err)
	}
	return nil
}

type stringerInstance struct {
	*kcinstances.GetResponseItem
}

var _ fmt.Stringer = (*stringerInstance)(nil)

// String implements fmt.Stringer.
func (i stringerInstance) String() string {
	return i.Name
}

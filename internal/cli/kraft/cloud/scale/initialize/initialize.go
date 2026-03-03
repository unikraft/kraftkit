// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package initialize

import (
	"context"
	"fmt"
	"strconv"
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
	AllowInsecure bool                  `noattribute:"true"`
	Auth          *config.AuthConfig    `noattribute:"true"`
	Client        kraftcloud.KraftCloud `noattribute:"true"`
	CooldownTime  time.Duration         `long:"cooldown-time" short:"c" usage:"The cooldown time of the config (ms/s/m/h)"`
	MaxSize       int                   `long:"max-size" short:"M" usage:"The maximum size of the configuration" default:"10"`
	Metro         string                `noattribute:"true"`
	MinSize       string                `long:"min-size" short:"m" usage:"The minimum size of the configuration"`
	Template      string                `long:"template" short:"t" usage:"The UUID or Name of the instance template"`
	Token         string                `noattribute:"true"`
	WarmupTime    time.Duration         `long:"warmup-time" short:"w" usage:"The warmup time of the config (ms/s/m/h)"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&InitOptions{}, cobra.Command{
		Short:   "Initialize autoscale configuration for a service",
		Use:     "init [FLAGS] NAME|UUID",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"init", "initialise", "initialize", "i"},
		Example: heredoc.Doc(`
			# Initialize an autoscale configuration
			$ kraft cloud scale init my-service \
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
		return fmt.Errorf("specify a service name or UUID")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token, &opts.AllowInsecure)
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
			kraftcloud.WithAllowInsecure(opts.AllowInsecure),
			kraftcloud.WithToken(config.GetKraftCloudTokenAuthConfig(*opts.Auth)),
		)
	}

	id := args[0]

	// Look up the configuration by name
	if !utils.IsUUID(id) {
		confResp, err := opts.Client.Services().WithMetro(opts.Metro).Get(ctx, id)
		if err != nil {
			return fmt.Errorf("could not get configuration: %w", err)
		}
		conf, err := confResp.FirstOrErr()
		if err != nil {
			return fmt.Errorf("could not get configuration: %w", err)
		}

		id = conf.UUID
	}

	if opts.WarmupTime > 0 && opts.WarmupTime < 10*time.Millisecond {
		return fmt.Errorf("warmup time must be at least 10ms")
	}

	if opts.CooldownTime > 0 && opts.CooldownTime < 10*time.Millisecond {
		return fmt.Errorf("cooldown time must be at least 10ms")
	}

	var template kcautoscale.CreateRequestTemplate

	if opts.Template == "" {
		if config.G[config.KraftKit](ctx).NoPrompt {
			return fmt.Errorf("specify an instance template UUID or name via --template")
		}

		instListResp, err := opts.Client.Instances().WithMetro(opts.Metro).ListTemplate(ctx)
		if err != nil {
			return fmt.Errorf("could not list instance templates: %w", err)
		}
		instList, err := instListResp.AllOrErr()
		if err != nil {
			return fmt.Errorf("could not list instance templates: %w", err)
		}
		if len(instList) == 0 {
			return fmt.Errorf("no instance template found in service")
		}

		if len(instList) == 1 {
			template.UUID = &instList[0].UUID
		} else {
			var possible []stringerInstance

			uuids := make([]string, 0, len(instList))
			for _, instItem := range instList {
				uuids = append(uuids, instItem.UUID)
			}

			instancesResp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, uuids...)
			if err != nil {
				return fmt.Errorf("getting details of %d instance(s): %w", len(instList), err)
			}
			instances, err := instancesResp.AllOrErr()
			if err != nil {
				return fmt.Errorf("getting details of %d instance(s): %w", len(instList), err)
			}

			for _, inst := range instances {
				possible = append(possible, stringerInstance{&inst})
			}

			result, err := selection.Select(
				"select instance template",
				possible...,
			)
			if err != nil {
				return fmt.Errorf("could not select instance template: %w", err)
			}

			template.UUID = &result.UUID
		}
	} else {
		if utils.IsUUID(opts.Template) {
			template.UUID = &opts.Template
		} else {
			template.Name = &opts.Template
		}
	}

	req := kcautoscale.CreateRequest{
		UUID: &id,
		CreateArgs: kcautoscale.CreateRequestCreateArgs{
			Template: &template,
		},
	}

	if opts.MinSize != "" {
		minSize, err := strconv.ParseUint(opts.MinSize, 10, 64)
		if err != nil {
			return fmt.Errorf("could not parse minimum size: %w", err)
		}
		minSizeInt := int(minSize)
		req.MinSize = &minSizeInt
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

	scaleResp, err := opts.Client.Autoscale().WithMetro(opts.Metro).CreateConfiguration(ctx, req)
	if err != nil {
		return fmt.Errorf("could not create configuration: %w", err)
	}
	if _, err = scaleResp.AllOrErr(); err != nil {
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

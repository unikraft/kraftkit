// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package get

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	cloud "sdk.kraft.cloud"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
)

type GetOptions struct {
	Auth   *config.AuthConfig `noattribute:"true"`
	Client cloud.KraftCloud   `noattribute:"true"`
	Metro  string             `noattribute:"true"`
	Output string             `long:"output" short:"o" usage:"Output format" default:"list"`
	Policy string             `long:"policy" short:"p" usage:"Get a policy instead of a configuration"`
	Token  string             `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&GetOptions{}, cobra.Command{
		Short:   "Get an autoscale configuration or policy",
		Use:     "get [FLAGS] UUID|NAME",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"gt"},
		Long:    "Get an autoscale configuration or policy of a service.",
		Example: heredoc.Doc(`
			# Get an autoscale configuration by UUID of a service
			$ kraft cloud scale get fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Get an autoscale configuration by name of a service
			$ kraft cloud scale get my-service

			# Get an autoscale policy by UUID of a service
			$ kraft cloud scale get fd1684ea-7970-4994-92d6-61dcc7905f2b --policy my-policy

			# Get an autoscale policy by name of a service
			$ kraft cloud scale get my-service --policy my-policy
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "cloud-scale",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *GetOptions) Pre(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("specify a service NAME or UUID")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *GetOptions) Run(ctx context.Context, args []string) error {
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

	if len(opts.Policy) > 0 {
		policyResp, err := opts.Client.Autoscale().WithMetro(opts.Metro).GetPolicy(ctx, id, opts.Policy)
		if err != nil {
			return fmt.Errorf("could not get configuration: %w", err)
		}

		policy, err := policyResp.FirstOrErr()
		if err != nil {
			return fmt.Errorf("could not get configuration: %w", err)
		}

		err = iostreams.G(ctx).StartPager()
		if err != nil {
			log.G(ctx).Errorf("error starting pager: %v", err)
		}

		defer iostreams.G(ctx).StopPager()

		var b []byte

		switch opts.Output {
		case string(tableprinter.OutputFormatJSON):
			b, err = json.Marshal(&policy)
			if err != nil {
				return fmt.Errorf("could not marshal policy: %w", err)
			}

		default:
			b, err = yaml.Marshal(&policy)
			if err != nil {
				return fmt.Errorf("could not marshal policy: %w", err)
			}
		}

		fmt.Fprintf(iostreams.G(ctx).Out, "%s\n", b)
		return nil

	} else {
		resp, err := opts.Client.Autoscale().WithMetro(opts.Metro).GetConfigurations(ctx, id)
		if err != nil {
			return fmt.Errorf("could not get configuration: %w", err)
		}

		return utils.PrintAutoscaleConfiguration(ctx, opts.Output, *resp)
	}
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package drain

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kraftcloud "sdk.kraft.cloud"
	kcclient "sdk.kraft.cloud/client"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/cloud/utils"
	"kraftkit.sh/log"
	"kraftkit.sh/tui/processtree"
)

type DrainOptions struct {
	AllowInsecure bool                  `noattribute:"true"`
	Auth          *config.AuthConfig    `noattribute:"true"`
	Client        kraftcloud.KraftCloud `noattribute:"true"`
	Metro         string                `noattribute:"true"`
	Token         string                `noattribute:"true"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&DrainOptions{}, cobra.Command{
		Short: "Drain instances in a service",
		Use:   "drain [FLAGS] [NAME|UUID [NAME|UUID]...]",
		Args:  cobra.ArbitraryArgs,
		Example: heredoc.Doc(`
			# Drain a service from your account by UUID.
			$ kraft cloud service drain fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Drain a service from your account by name.
			$ kraft cloud service drain my-service

			# Drain multiple service from your account.
			$ kraft cloud service drain my-service my-other-service
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-service",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *DrainOptions) Pre(cmd *cobra.Command, args []string) error {
	if err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token, &opts.AllowInsecure); err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *DrainOptions) Run(ctx context.Context, args []string) error {
	return Drain(ctx, opts, args...)
}

func Drain(ctx context.Context, opts *DrainOptions, args ...string) error {
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

	var processes []*processtree.ProcessTreeItem

	for _, service := range args {
		processes = append(processes,
			processtree.NewProcessTreeItem(
				fmt.Sprintf("draining %s", service),
				"",
				func(ctx context.Context) error {
					serviceResp, err := opts.Client.Services().WithMetro(opts.Metro).Get(ctx, service)
					if err != nil {
						return fmt.Errorf("could not get service: %w", err)
					}

					sg, err := serviceResp.FirstOrErr()
					if err != nil && *sg.Error == kcclient.APIHTTPErrorNotFound {
						return nil
					} else if err != nil {
						return err
					}

					if len(sg.Instances) == 0 {
						return nil
					}

					var instances []string
					for _, instance := range sg.Instances {
						instances = append(instances, instance.UUID)
					}

					log.G(ctx).Infof("deleting %d instances...", len(instances))

					if _, err := opts.Client.Instances().WithMetro(opts.Metro).Delete(ctx, instances...); err != nil {
						return err
					}

					// Wait until the instances are deleted
					for {
						resp, err := opts.Client.Instances().WithMetro(opts.Metro).Get(ctx, instances...)
						if err != nil {
							break
						}

						noError := true
						for _, entry := range resp.Data.Entries {
							if entry.Status != "error" {
								noError = false
							}
						}

						if noError {
							break
						}
					}

					return err
				},
			),
		)
	}

	treemodel, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(true),
			processtree.WithRenderer(
				log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY,
			),
			processtree.WithFailFast(true),
			processtree.WithHideOnSuccess(true),
		},
		processes...,
	)
	if err != nil {
		return err
	}

	return treemodel.Start()
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package remove

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

type RemoveOptions struct {
	All       bool                  `long:"all" short:"a" usage:"Remove all services"`
	Auth      *config.AuthConfig    `noattribute:"true"`
	Client    kraftcloud.KraftCloud `noattribute:"true"`
	Metro     string                `noattribute:"true"`
	Token     string                `noattribute:"true"`
	WaitEmpty bool                  `long:"wait-empty" usage:"Wait for the service to be empty before removing it"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&RemoveOptions{}, cobra.Command{
		Short:   "Delete services",
		Use:     "remove [FLAGS] [NAME|UUID [NAME|UUID]...]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"delete", "del", "rm"},
		Example: heredoc.Doc(`
			# Remove a service from your account by UUID.
			$ kraft cloud service remove fd1684ea-7970-4994-92d6-61dcc7905f2b

			# Remove a service from your account by name.
			$ kraft cloud service remove my-service

			# Remove multiple service from your account.
			$ kraft cloud service remove my-service my-other-service

			# Remove all service from your account.
			$ kraft cloud service remove --all
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-svc",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *RemoveOptions) Pre(cmd *cobra.Command, args []string) error {
	if !opts.All && len(args) == 0 {
		return fmt.Errorf("either specify an instance name or UUID, or use the --all flag")
	}

	err := utils.PopulateMetroToken(cmd, &opts.Metro, &opts.Token)
	if err != nil {
		return fmt.Errorf("could not populate metro and token: %w", err)
	}

	return nil
}

func (opts *RemoveOptions) Run(ctx context.Context, args []string) error {
	return Remove(ctx, opts, args...)
}

func Remove(ctx context.Context, opts *RemoveOptions, args ...string) error {
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

	if opts.All {
		sgListResp, err := opts.Client.Services().WithMetro(opts.Metro).List(ctx)
		if err != nil {
			return fmt.Errorf("listing service: %w", err)
		}

		sgList, err := sgListResp.AllOrErr()
		if err != nil {
			return fmt.Errorf("listing service: %w", err)
		}

		if len(sgList) == 0 {
			log.G(ctx).Info("no service found")
			return nil
		}

		args = []string{}
		attachedInstances := []string{}
		for _, sgItem := range sgList {
			if sgItem.Instances != nil && len(sgItem.Instances) > 0 {
				attachedInstances = append(attachedInstances, sgItem.Name)
			} else {
				args = append(args, sgItem.Name)
			}
		}

		if len(attachedInstances) > 0 {
			log.G(ctx).Warnf("ignoring %d service(s) as instances are attached: %v", len(attachedInstances), attachedInstances)
		}
	}

	if opts.WaitEmpty {
		var processes []*processtree.ProcessTreeItem

		services := args
		args = []string{}

		for _, service := range services {
			processes = append(processes,
				processtree.NewProcessTreeItem(
					fmt.Sprintf("waiting for %s to be empty", service),
					"",
					func(ctx context.Context) error {
						for {
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
								args = append(args, service)
								break
							}
						}

						return nil
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

		if err := treemodel.Start(); err != nil {
			return err
		}
	}

	if len(args) == 0 {
		return nil
	}

	log.G(ctx).Infof("removing %d service(s)", len(args))

	resp, err := opts.Client.Services().WithMetro(opts.Metro).Delete(ctx, args...)
	if err != nil {
		return fmt.Errorf("removing %d service(s): %w", len(args), err)
	}
	if _, err := resp.AllOrErr(); err != nil {
		return fmt.Errorf("removing %d service(s): %w", len(args), err)
	}

	return nil
}

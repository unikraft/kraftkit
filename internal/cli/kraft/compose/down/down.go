// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package down

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/compose-spec/compose-go/v2/types"

	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	networkremove "kraftkit.sh/internal/cli/kraft/net/remove"
	machineremove "kraftkit.sh/internal/cli/kraft/remove"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	mnetwork "kraftkit.sh/machine/network"
	mplatform "kraftkit.sh/machine/platform"
)

type DownOptions struct {
	composefile string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&DownOptions{}, cobra.Command{
		Short:   "Stop and remove a compose project",
		Use:     "down [FLAGS]",
		Aliases: []string{"dw"},
		Long:    "Stop and remove a compose project.",
		Example: heredoc.Doc(`
			# Stop and remove a compose project
			$ kraft compose down
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "compose",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *DownOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	if cmd.Flag("file").Changed {
		opts.composefile = cmd.Flag("file").Value.String()
	}

	log.G(cmd.Context()).WithField("composefile", opts.composefile).Debug("using")
	return nil
}

func (opts *DownOptions) Run(ctx context.Context, args []string) error {
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}
	project, err := compose.NewProjectFromComposeFile(ctx, workdir, opts.composefile)
	if err != nil {
		return err
	}

	if err := project.Validate(ctx); err != nil {
		return err
	}

	machineController, err := mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	if err != nil {
		return err
	}

	machines, err := machineController.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return err
	}

	topLevelRender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY
	oldLogType := config.G[config.KraftKit](ctx).Log.Type
	config.G[config.KraftKit](ctx).Log.Type = log.LoggerTypeToString(log.BASIC)
	defer func() {
		config.G[config.KraftKit](ctx).Log.Type = oldLogType
	}()

	processes := make([]*processtree.ProcessTreeItem, 0)
	for _, service := range project.Services {
		for _, machine := range machines.Items {
			if service.Name == machine.Name {
				processes = append(processes, processtree.NewProcessTreeItem(
					fmt.Sprintf("removing service %s", service.Name),
					"",
					func(ctx context.Context) error {
						return removeService(ctx, service)
					},
				))
			}
		}
	}

	networkController, err := mnetwork.NewNetworkV1alpha1ServiceIterator(ctx)
	if err != nil {
		return err
	}

	networks, err := networkController.List(ctx, &networkapi.NetworkList{})
	if err != nil {
		return err
	}

	for _, projectNetwork := range project.Networks {
		for _, network := range networks.Items {
			if projectNetwork.Name == network.Name {
				processes = append(processes, processtree.NewProcessTreeItem(
					fmt.Sprintf("removing network %s", projectNetwork.Name),
					"",
					func(ctx context.Context) error {
						return removeNetwork(ctx, projectNetwork)
					},
				))
			}
		}
	}

	model, err := processtree.NewProcessTree(ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(false),
			processtree.WithHideOnSuccess(false),
			processtree.WithRenderer(topLevelRender),
		},
		processes...,
	)
	if err != nil {
		return err
	}

	return model.Start()
}

func removeService(ctx context.Context, service types.ServiceConfig) error {
	removeOptions := machineremove.RemoveOptions{Platform: "auto"}

	return removeOptions.Run(ctx, []string{service.Name})
}

func removeNetwork(ctx context.Context, network types.NetworkConfig) error {
	driver := "bridge"
	if network.Driver != "" {
		driver = network.Driver
	}
	removeOptions := networkremove.RemoveOptions{Driver: driver}

	return removeOptions.Run(ctx, []string{network.Name})
}

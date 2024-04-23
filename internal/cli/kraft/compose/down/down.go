// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package down

import (
	"context"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/compose-spec/compose-go/v2/types"

	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/internal/cli/kraft/compose/utils"
	networkremove "kraftkit.sh/internal/cli/kraft/net/remove"
	machineremove "kraftkit.sh/internal/cli/kraft/remove"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	networkapi "kraftkit.sh/api/network/v1alpha1"
	mnetwork "kraftkit.sh/machine/network"
	mplatform "kraftkit.sh/machine/platform"
)

type DownOptions struct {
	composefile   string
	RemoveOrphans bool `long:"remove-orphans" usage:"Remove machines for services not defined in the Compose file."`
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

	if opts.RemoveOrphans {
		if err := utils.RemoveOrphans(ctx, project); err != nil {
			return err
		}
	}

	machineController, err := mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	if err != nil {
		return err
	}

	machines, err := machineController.List(ctx, &machineapi.MachineList{})
	if err != nil {
		return err
	}

	for _, service := range project.Services {
		for _, machine := range machines.Items {
			if service.Name == machine.Name {
				if err := removeService(ctx, service); err != nil {
					return err
				}
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
		if projectNetwork.External {
			continue
		}
		for _, network := range networks.Items {
			if projectNetwork.Name == network.Name {
				if err := removeNetwork(ctx, projectNetwork); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func removeService(ctx context.Context, service types.ServiceConfig) error {
	log.G(ctx).Infof("removing service %s...", service.Name)
	removeOptions := machineremove.RemoveOptions{Platform: "auto"}

	return removeOptions.Run(ctx, []string{service.Name})
}

func removeNetwork(ctx context.Context, network types.NetworkConfig) error {
	log.G(ctx).Infof("removing network %s...", network.Name)
	driver := "bridge"
	if network.Driver != "" {
		driver = network.Driver
	}
	removeOptions := networkremove.RemoveOptions{Driver: driver}

	return removeOptions.Run(ctx, []string{network.Name})
}

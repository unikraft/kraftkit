// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package down

import (
	"context"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/compose-spec/compose-go/types"

	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/internal/cli/kraft/remove"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
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

	controller, err := mplatform.NewMachineV1alpha1ServiceIterator(ctx)
	if err != nil {
		return err
	}

	machines, err := controller.List(ctx, &machineapi.MachineList{})
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

	return nil
}

func removeService(ctx context.Context, service types.ServiceConfig) error {
	log.G(ctx).Infof("removing service %s...", service.Name)
	removeOptions := remove.RemoveOptions{Platform: "auto"}

	return removeOptions.Run(ctx, []string{service.Name})
}

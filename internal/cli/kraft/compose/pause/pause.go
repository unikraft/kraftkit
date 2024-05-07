// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pause

import (
	"context"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	kernelpause "kraftkit.sh/internal/cli/kraft/pause"
	mplatform "kraftkit.sh/machine/platform"
)

type PauseOptions struct {
	composefile string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PauseOptions{}, cobra.Command{
		Short:   "Pause a compose project",
		Use:     "pause [FLAGS]",
		Aliases: []string{},
		Example: heredoc.Doc(`
			# Pause a compose project
			$ kraft compose pause 
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

func (opts *PauseOptions) Pre(cmd *cobra.Command, _ []string) error {
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

func (opts *PauseOptions) Run(ctx context.Context, args []string) error {
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

	services, err := project.GetServices(args...)
	if err != nil {
		return err
	}

	machinesToPause := []string{}
	for _, service := range services {
		for _, machine := range machines.Items {
			if service.ContainerName == machine.Name && machine.Status.State == machineapi.MachineStateRunning {
				machinesToPause = append(machinesToPause, machine.Name)
			}
		}
	}

	if len(machinesToPause) == 0 {
		return nil
	}

	kernelPauseOptions := kernelpause.PauseOptions{
		Platform: "auto",
	}

	return kernelPauseOptions.Run(ctx, machinesToPause)
}

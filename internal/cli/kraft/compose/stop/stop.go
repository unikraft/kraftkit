// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package stop

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
	kernelstop "kraftkit.sh/internal/cli/kraft/stop"
	mplatform "kraftkit.sh/machine/platform"
)

type StopOptions struct {
	Composefile string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StopOptions{}, cobra.Command{
		Short:   "Stop a compose project",
		Use:     "stop [FLAGS]",
		Aliases: []string{},
		Example: heredoc.Doc(`
			# Stop a compose project
			$ kraft compose stop 
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

func (opts *StopOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	if cmd.Flag("file").Changed {
		opts.Composefile = cmd.Flag("file").Value.String()
	}

	log.G(cmd.Context()).WithField("composefile", opts.Composefile).Debug("using")
	return nil
}

func (opts *StopOptions) Run(ctx context.Context, args []string) error {
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}

	project, err := compose.NewProjectFromComposeFile(ctx, workdir, opts.Composefile)
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

	machinesToStop := []string{}
	for _, service := range services {
		for _, machine := range machines.Items {
			if service.ContainerName == machine.Name &&
				(machine.Status.State == machineapi.MachineStateRunning ||
					machine.Status.State == machineapi.MachineStatePaused) {
				machinesToStop = append(machinesToStop, machine.Name)
			}
		}
	}

	if len(machinesToStop) == 0 {
		return nil
	}

	kernelStopOptions := kernelstop.StopOptions{
		Platform: "auto",
	}

	return kernelStopOptions.Run(ctx, machinesToStop)
}

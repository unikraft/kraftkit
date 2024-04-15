// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package unpause

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
	"kraftkit.sh/internal/cli/kraft/compose/logs"
	kernelstart "kraftkit.sh/internal/cli/kraft/start"
	mplatform "kraftkit.sh/machine/platform"
)

type UnpauseOptions struct {
	Composefile string `noattribute:"true"`
	Detach      bool   `long:"detach" short:"d" usage:"Run in background"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&UnpauseOptions{}, cobra.Command{
		Short:   "Unpause a compose project",
		Use:     "unpause [FLAGS]",
		Aliases: []string{},
		Example: heredoc.Doc(`
			# Unpause a compose project
			$ kraft compose unpause 
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

func (opts *UnpauseOptions) Pre(cmd *cobra.Command, _ []string) error {
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

func (opts *UnpauseOptions) Run(ctx context.Context, args []string) error {
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

	machinesToUnpause := []string{}
	for _, service := range services {
		for _, machine := range machines.Items {
			if service.Name == machine.Name {
				if machine.Status.State == machineapi.MachineStatePaused {
					machinesToUnpause = append(machinesToUnpause, machine.Name)
				}
			}
		}
	}

	kernelStartOptions := kernelstart.StartOptions{
		Detach:   true,
		Platform: "auto",
	}

	if err := kernelStartOptions.Run(ctx, machinesToUnpause); err != nil {
		return err
	}

	if opts.Detach {
		return nil
	}

	logsOptions := logs.LogsOptions{
		Composefile: opts.Composefile,
		Follow:      true,
	}

	return logsOptions.Run(ctx, []string{})
}

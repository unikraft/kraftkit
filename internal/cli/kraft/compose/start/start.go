// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package start

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/internal/cli/kraft/compose/logs"
	kernelstart "kraftkit.sh/internal/cli/kraft/start"
	mplatform "kraftkit.sh/machine/platform"
)

type StartOptions struct {
	Composefile string `noattribute:"true"`
	Detach      bool   `long:"detach" short:"d" usage:"Run in background"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&StartOptions{}, cobra.Command{
		Short:   "Start a compose project",
		Use:     "start [FLAGS]",
		Args:    cobra.NoArgs,
		Aliases: []string{},
		Example: heredoc.Doc(`
			# Start a compose project
			$ kraft compose start 
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

func (opts *StartOptions) Pre(cmd *cobra.Command, _ []string) error {
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

func (opts *StartOptions) Run(ctx context.Context, _ []string) error {
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
				if machine.Status.State == machineapi.MachineStateCreated || machine.Status.State == machineapi.MachineStateExited {
					processes = append(processes, processtree.NewProcessTreeItem(
						fmt.Sprintf("starting service %s", service.Name),
						"",
						func(ctx context.Context) error {
							kernelStartOptions := kernelstart.StartOptions{
								Detach:   true,
								Platform: "auto",
							}
							return kernelStartOptions.Run(ctx, []string{service.Name})
						},
					))
				}
			}
		}
	}

	if len(processes) != 0 {
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

		if err := model.Start(); err != nil {
			return err
		}
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

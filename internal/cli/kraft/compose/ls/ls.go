// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package ls

import (
	"context"
	"path/filepath"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/cli/kraft/ps"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"

	composeapi "kraftkit.sh/api/compose/v1"
	machineapi "kraftkit.sh/api/machine/v1alpha1"
	mplatform "kraftkit.sh/machine/platform"
	"kraftkit.sh/packmanager"
)

type LsOptions struct {
	ShowAll bool   `long:"all" short:"a" usage:"Show all projects (default shows just running)"`
	Output  string `long:"output" short:"o" usage:"Set output format" default:"table"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&LsOptions{}, cobra.Command{
		Short:   "List compose projects",
		Use:     "ls [FLAGS]",
		Aliases: []string{"list"},
		Long:    "List compose projects.",
		Example: heredoc.Doc(`
			# List all compose projects
			$ kraft compose ls
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

func (opts *LsOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *LsOptions) Run(ctx context.Context, args []string) error {
	controller, err := compose.NewComposeProjectV1(ctx)
	if err != nil {
		return err
	}

	projects, err := controller.List(ctx, &composeapi.ComposeList{})
	if err != nil {
		return err
	}

	err = iostreams.G(ctx).StartPager()
	if err != nil {
		log.G(ctx).Errorf("error starting pager: %v", err)
	}

	defer iostreams.G(ctx).StopPager()

	cs := iostreams.G(ctx).ColorScheme()

	table, err := tableprinter.NewTablePrinter(ctx,
		tableprinter.WithMaxWidth(iostreams.G(ctx).TerminalWidth()),
		tableprinter.WithOutputFormatFromString(opts.Output),
	)
	if err != nil {
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

	table.AddField("NAME", cs.Bold)
	table.AddField("STATUS", cs.Bold)
	table.AddField("COMPOSEFILE", cs.Bold)
	table.EndRow()

	if config.G[config.KraftKit](ctx).NoColor {
		ps.MachineStateColor = ps.MachineStateColorNil
	}

	// Heuristic to determine project status based on machine status
	statePriority := map[machineapi.MachineState]int{
		machineapi.MachineStateRunning: 0,
		machineapi.MachineStatePaused:  1,
		machineapi.MachineStateCreated: 2,
		machineapi.MachineStateExited:  3,
	}

	for _, project := range projects.Items {
		status := machineapi.MachineStateExited

		for _, projectMachine := range project.Status.Machines {
			for _, machine := range machines.Items {
				if projectMachine.Name != machine.Name {
					continue
				}

				if statePriority[machine.Status.State] < statePriority[status] {
					status = machine.Status.State
				}
			}
		}

		if !opts.ShowAll && status != machineapi.MachineStateRunning {
			continue
		}

		table.AddField(project.Name, nil)
		table.AddField(status.String(), ps.MachineStateColor[status])

		composefile := filepath.Join(project.Spec.Workdir, project.Spec.Composefile)
		table.AddField(composefile, nil)
		table.EndRow()
	}

	return table.Render(iostreams.G(ctx).Out)
}

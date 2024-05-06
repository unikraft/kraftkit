// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package ps

import (
	"context"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/compose"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	composeapi "kraftkit.sh/api/compose/v1"
	pslist "kraftkit.sh/internal/cli/kraft/ps"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
)

type PsOptions struct {
	Long    bool   `long:"long" short:"l" usage:"Show more information"`
	Orphans bool   `long:"orphans" usage:"Include orphaned services (default: true)" default:"true"`
	Output  string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	Quiet   bool   `long:"quiet" short:"q" usage:"Only display machine IDs"`
	ShowAll bool   `long:"all" short:"a" usage:"Show all machines (default shows just running)"`

	composefile string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PsOptions{}, cobra.Command{
		Short:   "List running services of current project",
		Use:     "ps [FLAGS]",
		Args:    cobra.NoArgs,
		Aliases: []string{},
		Long:    "List running services of current project.",
		Example: heredoc.Doc(`
			# List running services of current project
			$ kraft compose ps
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

func (opts *PsOptions) Pre(cmd *cobra.Command, _ []string) error {
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

func (opts *PsOptions) Run(ctx context.Context, args []string) error {
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

	pslistOptions := pslist.PsOptions{
		Long:    opts.Long,
		Output:  opts.Output,
		Quiet:   opts.Quiet,
		ShowAll: opts.ShowAll,
	}

	psTable, err := pslistOptions.PsTable(ctx)
	if err != nil {
		return err
	}

	controller, err := compose.NewComposeProjectV1(ctx)
	if err != nil {
		return err
	}

	embeddedProject, err := controller.Get(ctx, &composeapi.Compose{
		ObjectMeta: metav1.ObjectMeta{
			Name: project.Name,
		},
	})
	if err != nil {
		return err
	}

	filteredPsTable := []pslist.PsEntry{}
	for _, psEntry := range psTable {
		for _, machine := range embeddedProject.Status.Machines {
			orphaned := true
			for _, service := range project.Services {
				if service.ContainerName == machine.Name {
					orphaned = false
					break
				}
			}

			if orphaned && !opts.Orphans {
				continue
			}

			if psEntry.Name == machine.Name {
				filteredPsTable = append(filteredPsTable, psEntry)
			}
		}
	}

	return pslistOptions.PrintPsTable(ctx, filteredPsTable)
}

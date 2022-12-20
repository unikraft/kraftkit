// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package prepare

import (
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/unikraft/app"
)

type Prepare struct{}

func New() *cobra.Command {
	return cmdfactory.New(&Prepare{}, cobra.Command{
		Short:   "Prepare a Unikraft unikernel",
		Use:     "prepare [DIR]",
		Aliases: []string{"p"},
		Args:    cmdfactory.MaxDirArgs(1),
		Long: heredoc.Doc(`
			prepare a Unikraft unikernel`),
		Example: heredoc.Doc(`
			# Prepare the cwd project
			$ kraft build prepare

			# Prepare a project at a path
			$ kraft build prepare path/to/app`),
		Annotations: map[string]string{
			"help:group": "build",
		},
	})
}

func (opts *Prepare) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	workdir := ""

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := app.NewProjectOptions(
		nil,
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultConfigPath(),
		app.WithProjectResolvedPaths(true),
		app.WithProjectDotConfig(false),
	)
	if err != nil {
		return err
	}

	// Interpret the project directory
	project, err := app.NewProjectFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Prepare(ctx, nil)
}

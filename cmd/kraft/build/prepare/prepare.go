// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package prepare

import (
	"context"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/make"
	"kraftkit.sh/unikraft/app"
)

type PrepareOptions struct{}

func PrepareCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &PrepareOptions{}
	cmd, err := cmdutil.NewCmd(f, "prepare")
	if err != nil {
		panic("could not initialize 'kraft build prepare' command")
	}

	cmd.Short = "Prepare a Unikraft unikernel"
	cmd.Use = "prepare [DIR]"
	cmd.Aliases = []string{"p"}
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Doc(`
		prepare a Unikraft unikernel`)
	cmd.Example = heredoc.Doc(`
		# Prepare the cwd project
		$ kraft build prepare

		# Prepare a project at a path
		$ kraft build prepare path/to/app
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		workdir := ""

		if len(args) == 0 {
			workdir, err = os.Getwd()
			if err != nil {
				return err
			}
		} else {
			workdir = args[0]
		}

		return prepareRun(opts, workdir)
	}

	return cmd
}

func prepareRun(copts *PrepareOptions, workdir string) error {
	ctx := context.Background()

	// Initialize at least the configuration options for a project
	projectOpts, err := app.NewProjectOptions(
		nil,
		app.WithWorkingDirectory(workdir),
		app.WithDefaultConfigPath(),
		app.WithResolvedPaths(true),
		app.WithDotConfig(false),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := app.NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Prepare(
		make.WithContext(ctx),
	)
}

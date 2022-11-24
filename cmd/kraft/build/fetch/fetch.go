// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package fetch

import (
	"context"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/make"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
)

type FetchOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)

	// Command-line arguments
	Platform     string
	Architecture string
}

func FetchCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &FetchOptions{
		PackageManager: f.PackageManager,
	}

	cmd, err := cmdutil.NewCmd(f, "fetch")
	if err != nil {
		panic("could not initialize 'kraft build fetch' command")
	}

	cmd.Short = "Fetch a Unikraft unikernel's dependencies"
	cmd.Use = "fetch [DIR]"
	cmd.Aliases = []string{"f"}
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Doc(`
		Fetch a Unikraft unikernel's dependencies`)
	cmd.Example = heredoc.Doc(`
		# Fetch the cwd project
		$ kraft build fetch

		# Fetch a project at a path
		$ kraft build fetch path/to/app
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

		return fetchRun(opts, workdir)
	}

	return cmd
}

func fetchRun(copts *FetchOptions, workdir string) error {
	ctx := context.Background()
	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := app.NewProjectOptions(
		nil,
		app.WithWorkingDirectory(workdir),
		app.WithDefaultConfigPath(),
		app.WithPackageManager(&pm),
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

	return project.Fetch(
		make.WithContext(ctx),
	)
}

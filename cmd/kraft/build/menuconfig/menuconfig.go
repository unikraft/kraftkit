// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package menuconfig

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

type MenuConfigOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
}

func MenuConfigCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &MenuConfigOptions{
		PackageManager: f.PackageManager,
	}

	cmd, err := cmdutil.NewCmd(f, "menuconfig")
	if err != nil {
		panic("could not initialize 'kraft build menuconfig' command")
	}

	cmd.Short = "menuconfig open's Unikraft configuration editor TUI"
	cmd.Use = "menuconfig [DIR]"
	cmd.Aliases = []string{"m", "menu"}
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Doc(`
		Open Unikraft's configuration editor TUI`)
	cmd.Example = heredoc.Doc(`
		# Open the menuconfig in the cwd project
		$ kraft build menuconfig
		
		# Open the menuconfig for a project at a path
		$ kraft build menu path/to/app
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

		return menuConfigRun(opts, workdir)
	}

	return cmd
}

func menuConfigRun(mcopts *MenuConfigOptions, workdir string) error {
	ctx := context.Background()
	pm, err := mcopts.PackageManager()
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

	return project.Make(
		make.WithContext(ctx),
		make.WithTarget("menuconfig"),
	)
}

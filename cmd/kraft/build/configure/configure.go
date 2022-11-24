// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package configure

import (
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/exec"
	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/make"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/app"
)

type ConfigureOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	IO             *iostreams.IOStreams
}

func ConfigureCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &ConfigureOptions{
		PackageManager: f.PackageManager,
		IO:             f.IOStreams,
	}

	cmd, err := cmdutil.NewCmd(f, "configure")
	if err != nil {
		panic("could not initialize 'kraft build configure' command")
	}

	cmd.Short = "Configure a Unikraft unikernel its dependencies"
	cmd.Use = "configure [DIR]"
	cmd.Args = cmdutil.MaxDirArgs(1)
	cmd.Long = heredoc.Doc(`
		Configure a Unikraft unikernel its dependencies`)
	cmd.Example = heredoc.Doc(`
		# Configure the cwd project
		$ kraft build configure

		# Configure a project at a path
		$ kraft build configure path/to/app
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

		return configureRun(opts, workdir)
	}

	return cmd
}

func configureRun(copts *ConfigureOptions, workdir string) error {
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

	return project.Configure(
		make.WithExecOptions(
			exec.WithStdin(copts.IO.In),
		),
	)
}

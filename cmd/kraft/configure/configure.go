// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package configure

import (
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/unikraft/app"

	"kraftkit.sh/internal/cli"
)

type Configure struct{}

func New() *cobra.Command {
	return cli.New(&Configure{}, cobra.Command{
		Short: "Configure a Unikraft unikernel its dependencies",
		Use:   "configure [DIR]",
		Args:  cli.MaxDirArgs(1),
		Long: heredoc.Doc(`
			Configure a Unikraft unikernel its dependencies`),
		Example: heredoc.Doc(`
			# Configure the cwd project
			$ kraft build configure

			# Configure a project at a path
			$ kraft build configure path/to/app`),
		Annotations: map[string]string{
			"help:group": "build",
		},
	})
}

func (opts *Configure) Run(cmd *cobra.Command, args []string) error {
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

	return project.Configure(ctx)
}

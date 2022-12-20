// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package fetch

import (
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/unikraft/app"
)

type Fetch struct {
	Architecture string `long:"arch" short:"m" usage:""`
	Platform     string `long:"plat" short:"p" usage:""`
}

func New() *cobra.Command {
	return cmdfactory.New(&Fetch{}, cobra.Command{
		Short:   "Fetch a Unikraft unikernel's dependencies",
		Use:     "fetch [DIR]",
		Aliases: []string{"f"},
		Args:    cmdfactory.MaxDirArgs(1),
		Long: heredoc.Doc(`
			Fetch a Unikraft unikernel's dependencies`),
		Example: heredoc.Doc(`
			# Fetch the cwd project
			$ kraft build fetch

			# Fetch a project at a path
			$ kraft build fetch path/to/app`),
		Annotations: map[string]string{
			"help:group": "build",
		},
	})
}

func (opts *Fetch) Run(cmd *cobra.Command, args []string) error {
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
	project, err := app.NewProjectFromOptions(
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
		app.WithProjectResolvedPaths(true),
		app.WithProjectDotConfig(false),
	)
	if err != nil {
		return err
	}

	return project.Fetch(ctx, nil)
}

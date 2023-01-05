// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package menu

import (
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/make"
	"kraftkit.sh/unikraft/app"
)

type Menu struct{}

func New() *cobra.Command {
	return cmdfactory.New(&Menu{}, cobra.Command{
		Short:   "Open's Unikraft configuration editor TUI",
		Use:     "menu [DIR]",
		Aliases: []string{"m", "menuconfig"},
		Args:    cmdfactory.MaxDirArgs(1),
		Long: heredoc.Doc(`
			Open Unikraft's configuration editor TUI`),
		Example: heredoc.Doc(`
			# Open configuration editor in the cwd project
			$ kraft menu
			
			# Open configuration editor for a project at a path
			$ kraft build menu path/to/app`),
		Annotations: map[string]string{
			"help:group": "build",
		},
	})
}

func (opts *Menu) Run(cmd *cobra.Command, args []string) error {
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
		ctx,
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultKraftfiles(),
	)
	if err != nil {
		return err
	}

	return project.Make(
		ctx,
		nil,
		make.WithTarget("menuconfig"),
	)
}

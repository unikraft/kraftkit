// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package main

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cli"

	"kraftkit.sh/cmd/kraft/build"
	"kraftkit.sh/cmd/kraft/clean"
	"kraftkit.sh/cmd/kraft/configure"
	"kraftkit.sh/cmd/kraft/events"
	"kraftkit.sh/cmd/kraft/fetch"
	"kraftkit.sh/cmd/kraft/menu"
	"kraftkit.sh/cmd/kraft/pkg"
	"kraftkit.sh/cmd/kraft/ps"
	"kraftkit.sh/cmd/kraft/rm"
	"kraftkit.sh/cmd/kraft/run"
	"kraftkit.sh/cmd/kraft/stop"

	// Additional initializers
	_ "kraftkit.sh/manifest"
)

type Kraft struct{}

func New() *cobra.Command {
	cmd := cli.New(&Kraft{}, cobra.Command{
		Short: "Build and use highly customized and ultra-lightweight unikernels",
		Long: heredoc.Docf(`
        .
       /^\     Build and use highly customized and ultra-lightweight unikernels.
      :[ ]:
      | = |
     /|/=\|\   Documentation:    https://kraftkit.sh/
    (_:| |:_)  Issues & support: https://github.com/unikraft/kraftkit/issues
       v v
       ' '`),
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
	})

	cmd.AddGroup(
		&cobra.Group{
			ID:    "build",
			Title: "BUILD COMMANDS",
		},
		&cobra.Group{
			ID:    "pkg",
			Title: "PACKAGING COMMANDS",
		},
		&cobra.Group{
			ID:    "run",
			Title: "RUNTIME COMMANDS",
		},
	)

	cmd.AddCommand(build.New())
	cmd.AddCommand(clean.New())
	cmd.AddCommand(configure.New())
	cmd.AddCommand(events.New())
	cmd.AddCommand(fetch.New())
	cmd.AddCommand(menu.New())
	cmd.AddCommand(pkg.New())
	cmd.AddCommand(ps.New())
	cmd.AddCommand(rm.New())
	cmd.AddCommand(run.New())
	cmd.AddCommand(stop.New())

	return cmd
}

func (k *Kraft) Run(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func main() {
	cli.Main(New())
}

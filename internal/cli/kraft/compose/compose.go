// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package compose

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/cli/kraft/compose/build"
	"kraftkit.sh/internal/cli/kraft/compose/create"
	"kraftkit.sh/internal/cli/kraft/compose/down"
	"kraftkit.sh/internal/cli/kraft/compose/logs"
	"kraftkit.sh/internal/cli/kraft/compose/ls"
	"kraftkit.sh/internal/cli/kraft/compose/pause"
	"kraftkit.sh/internal/cli/kraft/compose/ps"
	"kraftkit.sh/internal/cli/kraft/compose/start"
	"kraftkit.sh/internal/cli/kraft/compose/stop"
	"kraftkit.sh/internal/cli/kraft/compose/unpause"
	"kraftkit.sh/internal/cli/kraft/compose/up"
)

type ComposeOptions struct {
	Composefile string `long:"file" short:"f" usage:"Set the Compose file."`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ComposeOptions{}, cobra.Command{
		Short:   "Build and run compose projects with Unikraft",
		Use:     "compose [FLAGS] [SUBCOMMAND|DIR]",
		Aliases: []string{},
		Long: heredoc.Docf(`
			Build and run compose projects with Unikraft.
		`),
		Example: heredoc.Doc(`
			# Start a compose project
			$ kraft compose up
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "compose",
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(build.NewCmd())
	cmd.AddCommand(create.NewCmd())
	cmd.AddCommand(down.NewCmd())
	cmd.AddCommand(logs.NewCmd())
	cmd.AddCommand(ls.NewCmd())
	cmd.AddCommand(pause.NewCmd())
	cmd.AddCommand(ps.NewCmd())
	cmd.AddCommand(start.NewCmd())
	cmd.AddCommand(stop.NewCmd())
	cmd.AddCommand(unpause.NewCmd())
	cmd.AddCommand(up.NewCmd())

	return cmd
}

func (opts *ComposeOptions) Pre(cmd *cobra.Command, _ []string) error {
	return nil
}

func (opts *ComposeOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}

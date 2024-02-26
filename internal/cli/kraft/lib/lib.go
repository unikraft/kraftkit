// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/cli/kraft/lib/add"
	"kraftkit.sh/internal/cli/kraft/lib/create"
	"kraftkit.sh/internal/cli/kraft/lib/remove"
)

type LibOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&LibOptions{}, cobra.Command{
		Short:   "Manage and maintain Unikraft microlibraries",
		Use:     "lib SUBCOMMAND",
		Aliases: []string{"library"},
		Long:    "Manage and maintain Unikraft microlibraries.",
		Example: heredoc.Doc(`
			# Add a new microlibrary to your project.
			$ kraft lib add
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "lib",
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(remove.NewCmd())
	cmd.AddCommand(add.NewCmd())
	cmd.AddCommand(create.NewCmd())

	return cmd
}

func (opts *LibOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}

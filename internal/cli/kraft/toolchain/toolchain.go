// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package toolchain

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/cli/kraft/toolchain/set"
)

type ToolchainOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ToolchainOptions{}, cobra.Command{
		Short:   "Manage the KraftKit toolchain configuration",
		Use:     "toolchain SUBCOMMAND",
		Aliases: []string{"tc"},
		Long:    "Manage build toolchain variables passed to Unikraft's build system.",
		Example: heredoc.Doc(`
			# Set a toolchain variable globally
			$ kraft toolchain set CC=/usr/bin/gcc-12

			# Set multiple toolchain variables at once
			$ kraft toolchain set CC=/usr/bin/gcc-12 UK_CFLAGS="-O2 -pipe"
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(set.NewCmd())
	// TODO: cmd.AddCommand(list.NewCmd())
	// TODO: cmd.AddCommand(unset.NewCmd())

	return cmd
}

func (opts *ToolchainOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/cli/kraft/lib/remove"
)

type Lib struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&Lib{}, cobra.Command{
		Short:   "Manage and maintain Unikraft microlibraries",
		Use:     "lib SUBCOMMAND",
		Aliases: []string{"library"},
		Hidden:  true,
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "lib",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(remove.NewCmd())

	return cmd
}

func (opts *Lib) Run(ctx context.Context, args []string) error {
	return pflag.ErrHelp
}

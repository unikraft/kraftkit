// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package metro

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/internal/cli/kraft/cloud/metro/list"

	"kraftkit.sh/cmdfactory"
)

type MetroOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&MetroOptions{}, cobra.Command{
		Short:   "Inspect Unikraft Cloud metros and regions",
		Use:     "metro",
		Aliases: []string{"metros", "m"},
		Example: heredoc.Doc(`
			# List metros available.
			$ kraft cloud metro list

			# List metros available in JSON format.
			$ kraft cloud metro list -o json

			# List metros available and their status.
			$ kraft cloud metro list --status
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "kraftcloud-metro",
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(list.NewCmd())

	return cmd
}

func (opts *MetroOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}

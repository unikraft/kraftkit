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

	"kraftkit.sh/internal/cli/kraft/cloud/compose/down"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/start"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/stop"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/up"

	"kraftkit.sh/cmdfactory"
)

type ComposeOptions struct {
	Composefile string `long:"file" usage:"Set the Compose file."`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ComposeOptions{}, cobra.Command{
		Short:   "Manage deployments on KraftCloud",
		Use:     "compose",
		Aliases: []string{"comp"},
		Long: heredoc.Doc(`
			Mange deployments KraftCloud.
		`),
		Example: heredoc.Doc(`
			# Start the current KraftCloud deployment.
			$ kraft cloud compose up

			# Stop the current KraftCloud deployment.
			$ kraft cloud compose down
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "kraftcloud-compose",
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(up.NewCmd())
	cmd.AddCommand(down.NewCmd())
	cmd.AddCommand(start.NewCmd())
	cmd.AddCommand(stop.NewCmd())

	return cmd
}

func (opts *ComposeOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}

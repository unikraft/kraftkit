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

	"kraftkit.sh/internal/cli/kraft/cloud/compose/build"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/create"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/down"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/list"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/logs"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/ps"
	"kraftkit.sh/internal/cli/kraft/cloud/compose/push"
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
		Short:   "Manage Compose deployments on Unikraft Cloud",
		Use:     "compose",
		Aliases: []string{"comp"},
		Long: heredoc.Doc(`
			Manage Compose deployments on Unikraft Cloud
		`),
		Example: heredoc.Doc(`
			# Deploy the Compose project on Unikraft Cloud
			$ kraft cloud compose up

			# Stop the deployment
			$ kraft cloud compose down

			# List the Compose services for this project Unikraft Cloud
			$ kraft cloud compose ps

			# Build a specific service image of a Compose project
			$ kraft cloud compose build nginx

			# Create a service image of a Compose project
			$ kraft cloud compose create

			# Push a service image of Compose project
			$ kraft cloud compose push nginx

			# View logs of a service deployment
			$ kraft cloud compose log nginx
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "cloud-compose",
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
	cmd.AddCommand(list.NewCmd())
	cmd.AddCommand(ps.NewCmd())
	cmd.AddCommand(build.NewCmd())
	cmd.AddCommand(create.NewCmd())
	cmd.AddCommand(push.NewCmd())
	cmd.AddCommand(logs.NewCmd())

	return cmd
}

func (opts *ComposeOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}

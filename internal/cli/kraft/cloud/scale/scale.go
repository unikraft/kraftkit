// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package scale

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/internal/cli/kraft/cloud/scale/add"
	"kraftkit.sh/internal/cli/kraft/cloud/scale/get"
	"kraftkit.sh/internal/cli/kraft/cloud/scale/initialize"
	"kraftkit.sh/internal/cli/kraft/cloud/scale/remove"
	"kraftkit.sh/internal/cli/kraft/cloud/scale/reset"

	"kraftkit.sh/cmdfactory"
)

type ScaleOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ScaleOptions{}, cobra.Command{
		Short:   "Manage instance autoscale policies",
		Use:     "scale SUBCOMMAND",
		Aliases: []string{"autoscale", "scl"},
		Example: heredoc.Doc(`
			# Add an autoscale configuration to a service
			$ kraft cloud scale add my-service my-policy
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "cloud-scale",
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(remove.NewCmd())
	cmd.AddCommand(add.NewCmd())
	cmd.AddCommand(reset.NewCmd())
	cmd.AddCommand(initialize.NewCmd())
	cmd.AddCommand(get.NewCmd())

	return cmd
}

func (opts *ScaleOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}

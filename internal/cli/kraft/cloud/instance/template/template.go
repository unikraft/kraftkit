// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package template

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/internal/cli/kraft/cloud/instance/template/create"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/template/get"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/template/list"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/template/remove"

	"kraftkit.sh/cmdfactory"
)

type TemplateOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&TemplateOptions{}, cobra.Command{
		Short:   "Manage templates on Unikraft Cloud",
		Use:     "template SUBCOMMAND",
		Aliases: []string{"templates"},
		Long:    "Manage templates on Unikraft Cloud.",
		Example: heredoc.Doc(`
			# Create a template in your account.
			$ kraft cloud instance template create my-template

			# Get a template in your account.
			$ kraft cloud instance template get my-template

			# List all templates in your account.
			$ kraft cloud instance template list

			# Delete a template in your account.
			$ kraft cloud instance template remove my-template
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "kraftcloud-instance-template",
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(create.NewCmd())
	cmd.AddCommand(get.NewCmd())
	cmd.AddCommand(list.NewCmd())
	cmd.AddCommand(remove.NewCmd())

	return cmd
}

func (opts *TemplateOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}

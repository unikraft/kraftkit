// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package service

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/internal/cli/kraft/cloud/service/create"
	"kraftkit.sh/internal/cli/kraft/cloud/service/get"
	"kraftkit.sh/internal/cli/kraft/cloud/service/list"
	"kraftkit.sh/internal/cli/kraft/cloud/service/remove"

	"kraftkit.sh/cmdfactory"
)

type ServiceOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&ServiceOptions{}, cobra.Command{
		Short:   "Manage services on UnikraftCloud",
		Use:     "service SUBCOMMAND",
		Aliases: []string{"services", "svc"},
		Long:    "Manage services on cloud.",
		Example: heredoc.Doc(`
			# List services in your account.
			$ kraft cloud service list
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "cloud-svc",
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(create.NewCmd())
	cmd.AddCommand(list.NewCmd())
	cmd.AddCommand(get.NewCmd())
	cmd.AddCommand(remove.NewCmd())

	return cmd
}

func (opts *ServiceOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package instance

import (
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"

	"kraftkit.sh/internal/cli/kraft/cloud/instance/create"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/list"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/logs"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/remove"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/start"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/status"
	"kraftkit.sh/internal/cli/kraft/cloud/instance/stop"
)

type InstanceOptions struct{}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&InstanceOptions{}, cobra.Command{
		Short:   "Manage KraftCloud instances",
		Use:     "instance SUBCOMMAND",
		Aliases: []string{"inst", "instances"},
		Hidden:  true,
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "kraftcloud-instance",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(create.New())
	cmd.AddCommand(list.New())
	cmd.AddCommand(logs.New())
	cmd.AddCommand(remove.New())
	cmd.AddCommand(start.New())
	cmd.AddCommand(status.New())
	cmd.AddCommand(stop.New())

	return cmd
}

func (opts *InstanceOptions) Run(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

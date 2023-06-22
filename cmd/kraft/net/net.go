// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package net

import (
	"fmt"

	"github.com/spf13/cobra"

	"kraftkit.sh/cmd/kraft/net/create"
	"kraftkit.sh/cmd/kraft/net/down"
	"kraftkit.sh/cmd/kraft/net/inspect"
	"kraftkit.sh/cmd/kraft/net/list"
	"kraftkit.sh/cmd/kraft/net/remove"
	"kraftkit.sh/cmd/kraft/net/up"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/machine/network"
)

type Net struct {
	Driver string `local:"false" long:"driver" short:"d" usage:"Set the network driver." default:"bridge"`
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Net{}, cobra.Command{
		Short:   "Manage machine networks",
		Use:     "net SUBCOMMAND",
		Aliases: []string{"network"},
		Hidden:  true,
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "net",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(create.New())
	cmd.AddCommand(down.New())
	cmd.AddCommand(inspect.New())
	cmd.AddCommand(list.New())
	cmd.AddCommand(remove.New())
	cmd.AddCommand(up.New())

	return cmd
}

func (opts *Net) Pre(cmd *cobra.Command, _ []string) error {
	if opts.Driver == "" {
		return fmt.Errorf("network driver must be set")
	} else if !set.NewStringSet(network.DriverNames()...).Contains(opts.Driver) {
		return fmt.Errorf("unsupported network driver strategy: %s", opts.Driver)
	}

	return nil
}

func (opts *Net) Run(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

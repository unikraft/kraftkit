// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package net

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/cli/kraft/net/create"
	"kraftkit.sh/internal/cli/kraft/net/down"
	"kraftkit.sh/internal/cli/kraft/net/inspect"
	"kraftkit.sh/internal/cli/kraft/net/list"
	"kraftkit.sh/internal/cli/kraft/net/remove"
	"kraftkit.sh/internal/cli/kraft/net/up"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/machine/network"
)

type NetOptions struct {
	Driver string `local:"false" long:"driver" short:"d" usage:"Set the network driver." default:"bridge"`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&NetOptions{}, cobra.Command{
		Short:   "Manage machine networks",
		Use:     "net SUBCOMMAND",
		Aliases: []string{"network"},
		Long:    "Manage machine networks.",
		Example: heredoc.Doc(`
			# Create a new network
			$ kraft network create my-network
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup:  "net",
			cmdfactory.AnnotationHelpHidden: "true",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(create.NewCmd())
	cmd.AddCommand(down.NewCmd())
	cmd.AddCommand(inspect.NewCmd())
	cmd.AddCommand(list.NewCmd())
	cmd.AddCommand(remove.NewCmd())
	cmd.AddCommand(up.NewCmd())

	return cmd
}

func (opts *NetOptions) Pre(cmd *cobra.Command, _ []string) error {
	if opts.Driver == "" {
		return fmt.Errorf("network driver must be set")
	} else if !set.NewStringSet(network.DriverNames()...).Contains(opts.Driver) {
		return fmt.Errorf("unsupported network driver strategy: %s", opts.Driver)
	}

	return nil
}

func (opts *NetOptions) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}

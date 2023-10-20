// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package x

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"kraftkit.sh/cmdfactory"

	"kraftkit.sh/internal/cli/kraft/x/probe"
)

type Exp struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&Exp{}, cobra.Command{
		Short:  "Experimental commands",
		Use:    "x [SUBCOMMAND]",
		Hidden: true,
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "experimental",
		},
	})
	if err != nil {
		panic(err)
	}

	cmd.AddCommand(probe.NewCmd())

	return cmd
}

func (opts *Exp) Pre(_ *cobra.Command, _ []string) error {
	return nil
}

func (opts *Exp) Run(_ context.Context, _ []string) error {
	return pflag.ErrHelp
}

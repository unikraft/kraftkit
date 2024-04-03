// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package version

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/version"
	"kraftkit.sh/iostreams"
)

type VersionOptions struct{}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&VersionOptions{}, cobra.Command{
		Short:   "Show kraft version information",
		Use:     "version",
		Aliases: []string{"v"},
		Args:    cobra.NoArgs,
		Long:    "Show kraft version information.",
		Example: heredoc.Doc(`
			# Show kraft version information
			$ kraft version
		`),
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *VersionOptions) Run(ctx context.Context, _ []string) error {
	fmt.Fprintf(iostreams.G(ctx).Out, "kraft %s", version.String())
	return nil
}

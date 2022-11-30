// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package source

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/packmanager"

	"kraftkit.sh/internal/cli"
)

type Source struct{}

func New() *cobra.Command {
	return cli.New(&Source{}, cobra.Command{
		Short: "Add Unikraft component manifests",
		Use:   "source [FLAGS] [SOURCE]",
		Args:  cli.MinimumArgs(1, "must specify component or manifest"),
		Example: heredoc.Docf(`
			# Add a single component as a Git repository
			$ kraft pkg source https://github.com/unikraft/unikraft.git

			# Add a manifest of components
			$ kraft pkg source https://raw.github.com/unikraft/index/stable/index.yaml`),
		Annotations: map[string]string{
			"help:group": "pkg",
		},
	})
}

func (opts *Source) Run(cmd *cobra.Command, args []string) error {
	var err error

	source := ""
	if len(args) > 0 {
		source = args[0]
	}

	ctx := cmd.Context()
	pm := packmanager.G(ctx)

	pm, err = pm.IsCompatible(ctx, source)
	if err != nil {
		return err
	}

	if err = pm.AddSource(ctx, source); err != nil {
		return err
	}

	return nil
}

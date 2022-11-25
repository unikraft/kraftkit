// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package source

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/internal/cmdfactory"
	"kraftkit.sh/internal/cmdutil"
	"kraftkit.sh/packmanager"
)

type SourceOptions struct{}

func SourceCmd(f *cmdfactory.Factory) *cobra.Command {
	opts := &SourceOptions{}
	cmd, err := cmdutil.NewCmd(f, "source")
	if err != nil {
		panic("could not initialize 'kraft pkg source' command")
	}

	cmd.Short = "Add Unikraft component manifests"
	cmd.Use = "source [FLAGS] [SOURCE]"
	cmd.Args = cmdutil.MinimumArgs(1, "must specify component or manifest")
	cmd.Aliases = []string{"a"}
	cmd.Long = heredoc.Docf(`
	`, "`")
	cmd.Example = heredoc.Docf(`
		# Add a single component as a Git repository
		$ kraft pkg source https://github.com/unikraft/unikraft.git

		# Add a manifest of components
		$ kraft pkg source https://raw.github.com/unikraft/index/stable/index.yaml
	`)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		source := ""
		if len(args) > 0 {
			source = args[0]
		}
		return sourceRun(opts, source)
	}

	return cmd
}

func sourceRun(opts *SourceOptions, source string) error {
	var err error

	ctx := context.Background()
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

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package source

import (
	"errors"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/packmanager"
)

type Source struct{}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Source{}, cobra.Command{
		Short: "Add Unikraft component manifests",
		Use:   "source [FLAGS] [SOURCE]",
		Args:  cmdfactory.MinimumArgs(1, "must specify component or manifest"),
		Example: heredoc.Docf(`
			# Add a single component as a Git repository
			$ kraft pkg source https://github.com/unikraft/unikraft.git

			# Add a manifest of components
			$ kraft pkg source https://raw.github.com/unikraft/index/stable/index.yaml`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (*Source) Pre(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

	return nil
}

func (opts *Source) Run(cmd *cobra.Command, args []string) error {
	var err error
	var compatible bool

	source := ""
	if len(args) > 0 {
		source = args[0]
	}

	ctx := cmd.Context()
	pm := packmanager.G(ctx)

	pm, compatible, err = pm.IsCompatible(ctx, source)
	if err != nil {
		return err
	} else if !compatible {
		return errors.New("incompatible package manager")
	}

	return pm.AddSource(ctx, source)
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package unsource implements the `kraft pkg unsource` command
package unsource

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/packmanager"
)

// Unsource is the command to remove a manifest pull location from the local config
type Unsource struct{}

// New returns a new unsource command
func New() *cobra.Command {
	return cmdfactory.New(&Unsource{}, cobra.Command{
		Short: "Remove Unikraft component manifests",
		Use:   "unsource [FLAGS] [SOURCE]",
		Args:  cmdfactory.MinimumArgs(1, "must specify component or manifest"),
		Example: heredoc.Docf(`
		# Remove a single component as a Git repository or manifest
		$ kraft pkg unsource https://github.com/unikraft/unikraft.git
		$ kraft pkg unsource https://manifests.kraftkit.sh/index.yaml
	`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
}

// Run executes the unsource command
func (opts *Unsource) Run(cmd *cobra.Command, args []string) error {
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

	return pm.RemoveSource(ctx, source)
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package unsource implements the `kraft pkg unsource` command
package unsource

import (
	"errors"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/packmanager"
)

// Unsource is the command to remove a manifest pull location from the local config
type Unsource struct{}

// New returns a new unsource command
func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Unsource{}, cobra.Command{
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
	if err != nil {
		panic(err)
	}

	return cmd
}

func (*Unsource) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

// Run executes the unsource command
func (opts *Unsource) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	for _, source := range args {
		_, compatible, err := packmanager.G(ctx).IsCompatible(ctx, source)
		if err != nil {
			return err
		} else if !compatible {
			return errors.New("incompatible package manager")
		}

		manifests := []string{}

		for _, manifest := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
			if source != manifest {
				manifests = append(manifests, manifest)
			}
		}

		config.G[config.KraftKit](ctx).Unikraft.Manifests = manifests

		if err := config.M[config.KraftKit](ctx).Write(false); err != nil {
			return err
		}
	}

	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package source

import (
	"context"
	"errors"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
)

type SourceOptions struct {
	Force bool `short:"F" long:"force" usage:"Do not run a compatibility test before sourcing."`
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&SourceOptions{}, cobra.Command{
		Short: "Add Unikraft component manifests",
		Use:   "source [FLAGS] [SOURCE]",
		Args:  cmdfactory.MinimumArgs(1, "must specify component or manifest"),
		Example: heredoc.Docf(`
			# Add a single component as a Git repository
			$ kraft pkg source https://github.com/unikraft/unikraft.git

			# Add a manifest of components
			$ kraft pkg source https://manifests.kraftkit.sh/index.yaml

			# Add a Unikraft-compatible OCI compatible registry
			$ kraft pkg source unikraft.org`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (*SourceOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *SourceOptions) Run(ctx context.Context, args []string) error {
	for _, source := range args {
		if !opts.Force {
			_, compatible, err := packmanager.G(ctx).IsCompatible(ctx,
				source,
				packmanager.WithUpdate(true),
			)
			if err != nil {
				return err
			} else if !compatible {
				return errors.New("incompatible package manager")
			}
		}

		for _, manifest := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
			if source == manifest {
				log.G(ctx).Warnf("manifest already saved: %s", source)
				return nil
			}
		}

		config.G[config.KraftKit](ctx).Unikraft.Manifests = append(
			config.G[config.KraftKit](ctx).Unikraft.Manifests,
			source,
		)

		if err := config.M[config.KraftKit](ctx).Write(true); err != nil {
			return err
		}
	}

	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package unsource implements the `kraft pkg unsource` command
package unsource

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
)

type UnsourceOptions struct{}

// NewCmd returns a new unsource command
func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&UnsourceOptions{}, cobra.Command{
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

// Unsource a remote location representing one-or-many Unikraft components.
func Unsource(ctx context.Context, opts *UnsourceOptions, args ...string) error {
	if opts == nil {
		opts = &UnsourceOptions{}
	}

	return opts.Run(ctx, args)
}

func (*UnsourceOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

// Run executes the unsource command
func (opts *UnsourceOptions) Run(ctx context.Context, args []string) error {
	for _, source := range args {
		manifests := []string{}

		var manifestRemoved bool
		for _, manifest := range config.G[config.KraftKit](ctx).Unikraft.Manifests {
			if source != manifest {
				manifests = append(manifests, manifest)
			} else {
				manifestRemoved = true
			}
		}

		if !manifestRemoved {
			log.G(ctx).Warnf("manifest not found: %s", source)
			return nil
		}

		config.G[config.KraftKit](ctx).Unikraft.Manifests = manifests

		if err := config.M[config.KraftKit](ctx).Write(false); err != nil {
			return err
		}
	}

	return nil
}

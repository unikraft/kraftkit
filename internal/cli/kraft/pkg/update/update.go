// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package update

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
)

type UpdateOptions struct {
	Manager string `long:"manager" short:"m" usage:"Force the handler type" default:"manifest" local:"true"`
}

// Update the local index of known locations for remote Unikraft components.
func Update(ctx context.Context, opts *UpdateOptions, args ...string) error {
	if opts == nil {
		opts = &UpdateOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&UpdateOptions{}, cobra.Command{
		Short:   "Retrieve new lists of Unikraft components, libraries and packages",
		Use:     "update [FLAGS]",
		Aliases: []string{"upd"},
		Long: heredoc.Doc(`
			Retrieve new lists of Unikraft components, libraries and packages.
		`),
		Example: heredoc.Doc(`
			# Update the local index of known locations for remote Unikraft components
			$ kraft pkg update
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

func (*UpdateOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *UpdateOptions) Run(ctx context.Context, args []string) error {
	var err error

	pm := packmanager.G(ctx)
	processes := []*processtree.ProcessTreeItem{}

	// Force a particular package manager
	if len(opts.Manager) > 0 && opts.Manager != "all" {
		pm, err = pm.From(pack.PackageFormat(opts.Manager))
		if err != nil {
			return err
		}

		processes = []*processtree.ProcessTreeItem{
			processtree.NewProcessTreeItem(
				"updating",
				pm.Format().String(),
				func(ctx context.Context) error {
					return pm.Update(ctx)
				},
			),
		}
	} else {
		umbrella, err := packmanager.PackageManagers()
		if err != nil {
			return err
		}
		for _, pm := range umbrella {
			processes = append(processes,
				processtree.NewProcessTreeItem(
					"updating",
					pm.Format().String(),
					func(ctx context.Context) error {
						return pm.Update(ctx)
					},
				),
			)
		}
	}

	model, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(!config.G[config.KraftKit](ctx).NoParallel),
			processtree.WithRenderer(log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY),
			processtree.WithHideOnSuccess(log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.JSON),
		},
		processes...,
	)
	if err != nil {
		return err
	}

	return model.Start()
}

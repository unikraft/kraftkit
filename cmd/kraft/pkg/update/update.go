// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package update

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"

	"kraftkit.sh/internal/cli"
)

type Update struct {
	Manager string `long:"manager" short:"m" usage:"Force the handler type" default:"manifest" local:"true"`
}

func New() *cobra.Command {
	return cli.New(&Update{}, cobra.Command{
		Short: "Retrieve new lists of Unikraft components, libraries and packages",
		Use:   "update [FLAGS]",
		Long: heredoc.Doc(`
			Retrieve new lists of Unikraft components, libraries and packages.`),
		Aliases: []string{"u"},
		Example: heredoc.Doc(`
			$ kraft pkg update
		`),
		Annotations: map[string]string{
			"help:group": "pkg",
		},
	})
}

func (opts *Update) Run(cmd *cobra.Command, args []string) error {
	var err error

	ctx := cmd.Context()
	pm := packmanager.G(ctx)

	// Force a particular package manager
	if len(opts.Manager) > 0 && opts.Manager != "auto" {
		pm, err = pm.From(opts.Manager)
		if err != nil {
			return err
		}
	}

	parallel := !config.G(ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G(ctx).Log.Type) != log.FANCY

	model, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			// processtree.WithVerb("Updating"),
			processtree.IsParallel(parallel),
			processtree.WithRenderer(norender),
		},
		[]*processtree.ProcessTreeItem{
			processtree.NewProcessTreeItem(
				"Updating...",
				"",
				func(ctx context.Context) error {
					return pm.Update(ctx)
				},
			),
		}...,
	)
	if err != nil {
		return err
	}

	if err := model.Start(); err != nil {
		return err
	}

	return nil
}

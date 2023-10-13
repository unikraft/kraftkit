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

type Update struct {
	Manager string `long:"manager" short:"m" usage:"Force the handler type" default:"manifest" local:"true"`
}

func New(cfg *config.ConfigManager[config.KraftKit]) *cobra.Command {
	cmd, err := cmdfactory.New(&Update{}, cobra.Command{
		Short: "Retrieve new lists of Unikraft components, libraries and packages",
		Use:   "update [FLAGS]",
		Long: heredoc.Doc(`
			Retrieve new lists of Unikraft components, libraries and packages.`),
		Aliases: []string{"u"},
		Example: heredoc.Doc(`
			$ kraft pkg update
		`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "pkg",
		},
	}, cfg)
	if err != nil {
		panic(err)
	}

	return cmd
}

func (*Update) Pre(cmd *cobra.Command, _ []string, cfg *config.ConfigManager[config.KraftKit]) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	return nil
}

func (opts *Update) Run(cmd *cobra.Command, args []string, cfgMgr *config.ConfigManager[config.KraftKit]) error {
	var err error

	ctx := cmd.Context()
	pm := packmanager.G(ctx)
	cfg := cfgMgr.Config

	// Force a particular package manager
	if len(opts.Manager) > 0 && opts.Manager != "auto" {
		pm, err = pm.From(pack.PackageFormat(opts.Manager))
		if err != nil {
			return err
		}
	}

	parallel := !cfg.NoParallel
	norender := log.LoggerTypeFromString(cfg.Log.Type) != log.FANCY

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
					return pm.Update(ctx, cfg)
				},
			),
		}...,
	)
	if err != nil {
		return err
	}

	return model.Start()
}

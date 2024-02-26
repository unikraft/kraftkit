// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package info

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	pkgutils "kraftkit.sh/internal/cli/kraft/pkg/utils"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/processtree"
)

type InfoOptions struct {
	Output string `long:"output" short:"o" usage:"Set output format. Options: table,yaml,json,list" default:"table"`
	Update bool   `long:"update" short:"u" usage:"Get latest information about components before listing results"`
}

// Info shows package information.
func Info(ctx context.Context, opts *InfoOptions, args ...string) error {
	if opts == nil {
		opts = &InfoOptions{}
	}

	return opts.Run(ctx, args)
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&InfoOptions{}, cobra.Command{
		Short:   "Show information about a package",
		Use:     "info [FLAGS] [PACKAGE|DIR]",
		Aliases: []string{"show", "get", "i"},
		Long: heredoc.Doc(`
			Shows a Unikraft package like library, core, etc.
		`),
		Args: cmdfactory.MinimumArgs(1, "package name(s) not specified"),
		Example: heredoc.Doc(`
			# Shows details for the library nginx
			$ kraft pkg info nginx
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

func (opts *InfoOptions) Run(ctx context.Context, args []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(ctx)
	if err != nil {
		return err
	}

	parallel := !config.G[config.KraftKit](ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	var searches []*processtree.ProcessTreeItem
	var packs []pack.Package

	for _, arg := range args {
		search := processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s", arg), "",
			func(ctx context.Context) error {
				more, err := packmanager.G(ctx).Catalog(ctx,
					packmanager.WithName(arg),
					packmanager.WithRemote(opts.Update),
				)
				if err != nil {
					return err
				}

				if len(more) == 0 {
					return fmt.Errorf("could not find: %s", arg)
				}

				packs = append(packs, more...)

				return nil
			},
		)

		searches = append(searches, search)
	}

	treemodel, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(parallel),
			processtree.WithRenderer(norender),
			processtree.WithFailFast(false),
			processtree.WithHideOnSuccess(true),
		},
		searches...,
	)
	if err != nil {
		return err
	}

	if err := treemodel.Start(); err != nil {
		return fmt.Errorf("could not complete search: %v", err)
	}

	if len(packs) == 0 {
		return fmt.Errorf("could not find package(s): %v", args)
	}

	return pkgutils.PrintPackages(ctx, iostreams.G(ctx).Out, opts.Output, packs...)
}

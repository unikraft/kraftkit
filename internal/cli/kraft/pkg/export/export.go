// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package export

import (
	"context"
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
)

type ExportOptions struct {
	Architecture string `long:"arch" short:"m" usage:"Specify the desired architecture"`
	Format       string `long:"as" short:"M" usage:"Force the packaging despite possible conflicts" default:"auto"`
	Output       string `long:"output" short:"o" usage:"Set the location to export the package to"`
	Platform     string `long:"plat" short:"p" usage:"Specify the desired platform"`
	Update       bool   `long:"update" short:"u" usage:"Fetch the latest information about components and pull if not present"`
}

// Export the package to a specified location.
func Info(ctx context.Context, opts *ExportOptions, args ...string) error {
	if opts == nil {
		opts = &ExportOptions{}
	}

	return opts.Run(ctx, args)
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&ExportOptions{}, cobra.Command{
		Short:   "Export a package",
		Use:     "export [FLAGS] PACKAGE",
		Aliases: []string{"e"},
		Long: heredoc.Doc(`
			Export a package to a specified location. The meaning of "export" is
			up to the implementing package, but it should be a representation of the
			package that can be used by other tools or processes.
		`),
		Args: cmdfactory.ExactArgs(1, "package name not specified"),
		Example: heredoc.Doc(`
			# Exports the NGINX OCI package to a tarball.
			$ kraft pkg export unikraft.org/nginx:1.15 -M oci -o ./output.tar.gz
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

func (opts *ExportOptions) Run(ctx context.Context, args []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(ctx)
	if err != nil {
		return err
	}

	parallel := !config.G[config.KraftKit](ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY
	pm := packmanager.G(ctx)

	// Force a particular package manager
	if len(opts.Format) > 0 && opts.Format != "auto" {
		pm, err = pm.From(pack.PackageFormat(opts.Format))
		if err != nil {
			return err
		}
	}

	var searches []*processtree.ProcessTreeItem
	var packs []pack.Package

	for _, arg := range args {
		search := processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s", arg), "",
			func(ctx context.Context) error {
				more, err := pm.Catalog(ctx,
					packmanager.WithArchitecture(opts.Architecture),
					packmanager.WithLocal(true),
					packmanager.WithName(arg),
					packmanager.WithPlatform(opts.Platform),
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

	if len(packs) > 1 {
		options := make([]string, len(packs))
		for i, p := range packs {
			options[i] = p.ID()
		}
		return fmt.Errorf("too many options: %v", options)
	}

	if opts.Update {
		proc := paraprogress.NewProcess(
			fmt.Sprintf("pulling %s", packs[0]),
			func(ctx context.Context, w func(progress float64)) error {
				return packs[0].Pull(
					ctx,
					pack.WithPullProgressFunc(w),
				)
			},
		)

		paramodel, err := paraprogress.NewParaProgress(
			ctx,
			[]*paraprogress.Process{proc},
			paraprogress.IsParallel(parallel),
			paraprogress.WithRenderer(norender),
			paraprogress.WithFailFast(true),
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return fmt.Errorf("could not pull all components: %v", err)
		}
	}

	return packs[0].Export(ctx, opts.Output)
}

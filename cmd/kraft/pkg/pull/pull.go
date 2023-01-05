// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pull

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
)

type Pull struct {
	AllVersions  bool   `long:"all-versions" short:"A" usage:"Pull all versions"`
	Architecture string `long:"arch" short:"m" usage:"Specify the desired architecture"`
	Manager      string `long:"manager" short:"M" usage:"Force the handler type (Omittion will attempt auto-detect)" default:"auto"`
	NoCache      bool   `long:"no-cache" short:"Z" usage:"Do not use cache and pull directly from source"`
	NoChecksum   bool   `long:"no-checksum" short:"C" usage:"Do not verify package checksum (if available)"`
	NoDeps       bool   `long:"no-deps" short:"D" usage:"Do not pull dependencies"`
	Platform     string `long:"plat" short:"p" usage:"Specify the desired platform"`
	WithDeps     bool   `long:"with-deps" short:"d" usage:"Pull dependencies"`
	Workdir      string `long:"workdir" short:"w" usage:"Set a path to working directory to pull components to"`
}

func New() *cobra.Command {
	return cmdfactory.New(&Pull{}, cobra.Command{
		Short:   "Pull a Unikraft unikernel and/or its dependencies",
		Use:     "pull [FLAGS] [PACKAGE|DIR]",
		Aliases: []string{"p"},
		Long: heredoc.Doc(`
			Pull a Unikraft unikernel, component microlibrary from a remote location
		`),
		Example: heredoc.Doc(`
			# Pull the dependencies for a project in the current working directory
			$ kraft pkg pull
			
			# Pull dependencies for a project at a path
			$ kraft pkg pull path/to/app

			# Pull a source repository
			$ kraft pkg pull github.com/unikraft/app-nginx.git

			# Pull from a manifest
			$ kraft pkg pull nginx:1.21.6`),
		Annotations: map[string]string{
			"help:group": "pkg",
		},
	})
}

func (opts *Pull) Pre(cmd *cobra.Command, args []string) error {
	return cmdfactory.MutuallyExclusive(
		"the `--with-deps` option is not supported with `--no-deps`",
		opts.WithDeps,
		opts.NoDeps,
	)
}

func (opts *Pull) Run(cmd *cobra.Command, args []string) error {
	var err error
	var project *app.ApplicationConfig
	var processes []*paraprogress.Process
	var queries []packmanager.CatalogQuery

	query := ""
	if len(args) > 0 {
		query = strings.Join(args, " ")
	}

	if len(query) == 0 {
		query, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	workdir := opts.Workdir
	ctx := cmd.Context()
	pm := packmanager.G(ctx)
	parallel := !config.G(ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G(ctx).Log.Type) != log.FANCY

	// Force a particular package manager
	if len(opts.Manager) > 0 && opts.Manager != "auto" {
		pm, err = pm.From(opts.Manager)
		if err != nil {
			return err
		}
	}

	// Are we pulling an application directory?  If so, interpret the application
	// so we can get a list of components
	if f, err := os.Stat(query); err == nil && f.IsDir() {
		workdir = query
		project, err := app.NewProjectFromOptions(
			ctx,
			app.WithProjectWorkdir(workdir),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		_, err = project.Components()
		if err != nil {
			// Pull the template from the package manager
			var packages []pack.Package
			search := processtree.NewProcessTreeItem(
				fmt.Sprintf("finding %s/%s:%s...", project.Template().Type(), project.Template().Name(), project.Template().Version()), "",
				func(ctx context.Context) error {
					packages, err = pm.Catalog(ctx, packmanager.CatalogQuery{
						Name:    project.Template().Name(),
						Types:   []unikraft.ComponentType{unikraft.ComponentTypeApp},
						Version: project.Template().Version(),
						NoCache: opts.NoCache,
					})
					if err != nil {
						return err
					}

					if len(packages) == 0 {
						return fmt.Errorf("could not find: %s", project.Template().Name())
					} else if len(packages) > 1 {
						return fmt.Errorf("too many options for %s", project.Template().Name())
					}
					return nil
				},
			)

			treemodel, err := processtree.NewProcessTree(
				ctx,
				[]processtree.ProcessTreeOption{
					processtree.IsParallel(parallel),
					processtree.WithRenderer(norender),
					processtree.WithFailFast(true),
				},
				[]*processtree.ProcessTreeItem{search}...,
			)
			if err != nil {
				return err
			}

			if err := treemodel.Start(); err != nil {
				return fmt.Errorf("could not complete search: %v", err)
			}

			proc := paraprogress.NewProcess(
				fmt.Sprintf("pulling %s", packages[0].Options().TypeNameVersion()),
				func(ctx context.Context, w func(progress float64)) error {
					return packages[0].Pull(
						ctx,
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(workdir),
						// pack.WithPullChecksum(!opts.NoChecksum),
						// pack.WithPullCache(!opts.NoCache),
					)
				},
			)

			processes = append(processes, proc)

			paramodel, err := paraprogress.NewParaProgress(
				ctx,
				processes,
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

		templateWorkdir, err := unikraft.PlaceComponent(workdir, project.Template().Type(), project.Template().Name())
		if err != nil {
			return err
		}

		templateProject, err := app.NewProjectFromOptions(
			ctx,
			app.WithProjectWorkdir(templateWorkdir),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		project = templateProject.MergeTemplate(project)
		// List the components
		components, err := project.Components()
		if err != nil {
			return err
		}
		for _, c := range components {
			queries = append(queries, packmanager.CatalogQuery{
				Name:    c.Name(),
				Version: c.Version(),
				Types:   []unikraft.ComponentType{c.Type()},
			})
		}

		// Is this a list (space delimetered) of packages to pull?
	} else {
		for _, c := range strings.Split(query, " ") {
			query := packmanager.CatalogQuery{}
			t, n, v, err := unikraft.GuessTypeNameVersion(c)
			if err != nil {
				continue
			}

			if t != unikraft.ComponentTypeUnknown {
				query.Types = append(query.Types, t)
			}

			if len(n) > 0 {
				query.Name = n
			}

			if len(v) > 0 {
				query.Version = v
			}

			queries = append(queries, query)
		}
	}

	for _, c := range queries {
		next, err := pm.Catalog(ctx, c)
		if err != nil {
			return err
		}

		if len(next) == 0 {
			log.G(ctx).Warnf("could not find %s", c.String())
			continue
		}

		for _, p := range next {
			p := p
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("pulling %s", p.Options().TypeNameVersion()),
				func(ctx context.Context, w func(progress float64)) error {
					return p.Pull(
						ctx,
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(workdir),
						pack.WithPullChecksum(!opts.NoChecksum),
						pack.WithPullCache(!opts.NoCache),
					)
				},
			))
		}
	}

	model, err := paraprogress.NewParaProgress(
		ctx,
		processes,
		paraprogress.IsParallel(parallel),
		paraprogress.WithRenderer(norender),
		paraprogress.WithFailFast(true),
	)
	if err != nil {
		return err
	}

	if err := model.Start(); err != nil {
		return err
	}

	if project != nil {
		fmt.Fprint(iostreams.G(ctx).Out, project.PrintInfo())
	}

	return nil
}

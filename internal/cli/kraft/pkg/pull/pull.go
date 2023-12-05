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
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/arch"
)

type PullOptions struct {
	All          bool     `long:"all" short:"A" usage:"Pull all versions"`
	Architecture string   `long:"arch" short:"m" usage:"Specify the desired architecture"`
	ForceCache   bool     `long:"force-cache" short:"Z" usage:"Force using cache and pull directly from source"`
	KConfig      []string `long:"kconfig" short:"k" usage:"Request a package with specific KConfig options."`
	Kraftfile    string   `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	Manager      string   `long:"as" short:"M" usage:"Force the handler type (Omitting it will attempt auto-detect)" default:"auto"`
	NoChecksum   bool     `long:"no-checksum" short:"C" usage:"Do not verify package checksum (if available)"`
	NoDeps       bool     `long:"no-deps" short:"D" usage:"Do not pull dependencies"`
	Output       string   `long:"output" short:"o" usage:"Save the package contents to the provided directory"`
	Platform     string   `long:"plat" short:"p" usage:"Specify the desired platform"`
	WithDeps     bool     `long:"with-deps" short:"d" usage:"Pull dependencies"`
	Workdir      string   `long:"workdir" short:"w" usage:"Set a path to working directory to pull components to"`
}

// Pull a Unikraft component.
func Pull(ctx context.Context, opts *PullOptions, args ...string) error {
	if opts == nil {
		opts = &PullOptions{}
	}

	return opts.Run(ctx, args)
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&PullOptions{}, cobra.Command{
		Short:   "Pull a Unikraft unikernel and/or its dependencies",
		Use:     "pull [FLAGS] [PACKAGE|DIR]",
		Aliases: []string{"pl"},
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
			$ kraft pkg pull nginx:1.21.6

			# Pull from a registry
			$ kraft pkg pull unikraft.org/nginx:1.21.6
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

func (opts *PullOptions) Pre(cmd *cobra.Command, _ []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	if strings.ContainsRune(opts.Platform, '/') && opts.Architecture == "" {
		split := strings.SplitN(opts.Platform, "/", 2)
		if len(split) != 2 {
			return fmt.Errorf("expected the flag in the form --plat=<plat>/<arch>")
		}
		opts.Platform = split[0]
		opts.Architecture = split[1]
	}

	opts.Platform = platform.PlatformByName(opts.Platform).String()

	return cmdfactory.MutuallyExclusive(
		"the `--with-deps` option is not supported with `--no-deps`",
		opts.WithDeps,
		opts.NoDeps,
	)
}

func (opts *PullOptions) Run(ctx context.Context, args []string) error {
	var err error
	var project app.Application
	var processes []*paraprogress.Process

	if len(opts.Workdir) == 0 {
		opts.Workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	if len(opts.Output) == 0 {
		opts.Output = opts.Workdir
	}

	pm := packmanager.G(ctx)
	parallel := !config.G[config.KraftKit](ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	// Force a particular package manager
	if len(opts.Manager) > 0 && opts.Manager != "auto" {
		pm, err = pm.From(pack.PackageFormat(opts.Manager))
		if err != nil {
			return err
		}
	}

	type pmQuery struct {
		pm    packmanager.PackageManager
		query []packmanager.QueryOption
	}

	// If `--all` is not set and either `--plat` or `--arch` are not set,
	// use the host platform and architecture, as the user is likely trying
	// to pull for their system by using "sensible defaults".
	if !opts.All {
		if opts.Architecture == "" {
			opts.Architecture, err = arch.HostArchitecture()
			if err != nil {
				return fmt.Errorf("could not determine host architecture: %w", err)
			}
		}

		if opts.Platform == "" {
			platform, _, err := platform.Detect(ctx)
			if err != nil {
				return fmt.Errorf("could not detect host platform: %w", err)
			}
			opts.Platform = platform.String()
		}
	}

	var queries []pmQuery

	// Are we pulling an application directory?  If so, interpret the application
	// so we can get a list of components
	if f, err := os.Stat(args[0]); err == nil && f.IsDir() {
		log.G(ctx).Debug("ignoring -w|--workdir")
		opts.Workdir = args[0]
		popts := []app.ProjectOption{}

		if len(opts.Kraftfile) > 0 {
			popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
		} else {
			popts = append(popts, app.WithProjectDefaultKraftfiles())
		}

		project, err := app.NewProjectFromOptions(
			ctx,
			append(popts, app.WithProjectWorkdir(opts.Workdir))...,
		)
		if err != nil {
			return err
		}

		if _, err = project.Components(ctx); err != nil {
			// Pull the template from the package manager
			var packages []pack.Package
			search := processtree.NewProcessTreeItem(
				fmt.Sprintf("finding %s",
					unikraft.TypeNameVersion(project.Template()),
				), "",
				func(ctx context.Context) error {
					qopts := []packmanager.QueryOption{
						packmanager.WithName(project.Template().Name()),
						packmanager.WithTypes(unikraft.ComponentTypeApp),
						packmanager.WithVersion(project.Template().Version()),
						packmanager.WithUpdate(opts.ForceCache),
						packmanager.WithPlatform(opts.Platform),
						packmanager.WithArchitecture(opts.Architecture),
					}
					packages, err = pm.Catalog(ctx, qopts...)
					if err != nil {
						return err
					}

					if len(packages) == 0 {
						return fmt.Errorf("could not find: %s based on %s", unikraft.TypeNameVersion(project.Template()), packmanager.NewQuery(qopts...).String())
					} else if len(packages) > 1 {
						return fmt.Errorf("too many options for %s", unikraft.TypeNameVersion(project.Template()))
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
				fmt.Sprintf("pulling %s",
					unikraft.TypeNameVersion(packages[0]),
				),
				func(ctx context.Context, w func(progress float64)) error {
					return packages[0].Pull(
						ctx,
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(opts.Output),
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

		templateWorkdir, err := unikraft.PlaceComponent(opts.Output, project.Template().Type(), project.Template().Name())
		if err != nil {
			return err
		}

		templateProject, err := app.NewProjectFromOptions(
			ctx,
			append(popts, app.WithProjectWorkdir(templateWorkdir))...,
		)
		if err != nil {
			return err
		}

		project, err = templateProject.MergeTemplate(ctx, project)
		if err != nil {
			return err
		}

		// List the components
		components, err := project.Components(ctx)
		if err != nil {
			return err
		}
		for _, c := range components {
			queries = append(queries, pmQuery{
				pm: pm,
				query: []packmanager.QueryOption{
					packmanager.WithName(c.Name()),
					packmanager.WithVersion(c.Version()),
					packmanager.WithSource(c.Source()),
					packmanager.WithTypes(c.Type()),
					packmanager.WithUpdate(!opts.ForceCache),
					packmanager.WithPlatform(opts.Platform),
					packmanager.WithArchitecture(opts.Architecture),
				},
			})
		}

		// Is this a list (space delimetered) of packages to pull?
	} else if len(args) > 0 {
		for _, arg := range args {
			pm, compatible, err := pm.IsCompatible(ctx, arg,
				packmanager.WithUpdate(!opts.ForceCache),
			)
			if err != nil || !compatible {
				continue
			}

			queries = append(queries, pmQuery{
				pm: pm,
				query: []packmanager.QueryOption{
					packmanager.WithUpdate(!opts.ForceCache),
					packmanager.WithName(arg),
					packmanager.WithArchitecture(opts.Architecture),
					packmanager.WithPlatform(opts.Platform),
					packmanager.WithKConfig(opts.KConfig),
				},
			})
		}
	}

	for _, c := range queries {
		query := packmanager.NewQuery(c.query...)
		next, err := c.pm.Catalog(ctx, c.query...)
		if err != nil {
			log.G(ctx).
				WithField("format", pm.Format().String()).
				WithField("name", query.Name()).
				Warn(err)
			continue
		}

		if len(next) == 0 {
			log.G(ctx).Warnf("could not find %s", query.String())
			continue
		}

		for _, p := range next {
			p := p
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("pulling %s", query.String()),
				func(ctx context.Context, w func(progress float64)) error {
					return p.Pull(
						ctx,
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(opts.Output),
						pack.WithPullChecksum(!opts.NoChecksum),
						pack.WithPullCache(opts.ForceCache),
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
		paraprogress.WithFailFast(false),
	)
	if err != nil {
		return err
	}

	if err := model.Start(); err != nil {
		return err
	}

	if project != nil {
		fmt.Fprint(iostreams.G(ctx).Out, project.PrintInfo(ctx))
	}

	return nil
}

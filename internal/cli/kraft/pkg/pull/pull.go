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
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/arch"
)

type PullOptions struct {
	All          bool     `long:"all" short:"A" usage:"Pull all versions"`
	Architecture string   `long:"arch" short:"m" usage:"Specify the desired architecture"`
	Format       string   `long:"as" short:"f" usage:"Set the package format" default:"auto"`
	KConfig      []string `long:"kconfig" short:"k" usage:"Request a package with specific KConfig options."`
	Kraftfile    string   `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	NoChecksum   bool     `long:"no-checksum" short:"C" usage:"Do not verify package checksum (if available)"`
	Output       string   `long:"output" short:"o" usage:"Save the package contents to the provided directory"`
	Platform     string   `long:"plat" short:"p" usage:"Specify the desired platform"`
	Update       bool     `long:"update" short:"u" usage:"Perform an update which gathers remote sources"`
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

	return nil
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

	if len(args) == 0 {
		args = []string{opts.Workdir}
	}

	pm := packmanager.G(ctx)
	parallel := !config.G[config.KraftKit](ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	// Force a particular package manager
	if len(opts.Format) > 0 && opts.Format != "auto" {
		pm, err = pm.From(pack.PackageFormat(opts.Format))
		if err != nil {
			return err
		}
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

	var queries [][]packmanager.QueryOption

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
			var pullPack pack.Package
			var packages []pack.Package

			// Pull the template from the package manager
			if project.Template() != nil {
				search := processtree.NewProcessTreeItem(
					fmt.Sprintf("finding %s",
						unikraft.TypeNameVersion(project.Template()),
					), "",
					func(ctx context.Context) error {
						qopts := []packmanager.QueryOption{
							packmanager.WithName(project.Template().Name()),
							packmanager.WithTypes(unikraft.ComponentTypeApp),
							packmanager.WithVersion(project.Template().Version()),
							packmanager.WithRemote(opts.Update),
							packmanager.WithPlatform(opts.Platform),
							packmanager.WithArchitecture(opts.Architecture),
							packmanager.WithLocal(true),
						}
						packages, err = pm.Catalog(ctx, qopts...)
						if err != nil {
							return err
						}

						if len(packages) == 0 {
							return fmt.Errorf("could not find: %s based on %s", unikraft.TypeNameVersion(project.Template()), packmanager.NewQuery(qopts...).String())
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

				if len(packages) == 1 {
					pullPack = packages[0]
				} else if len(packages) > 1 {
					if config.G[config.KraftKit](ctx).NoPrompt {
						for _, p := range packages {
							log.G(ctx).
								WithField("template", p.String()).
								Warn("possible")
						}

						return fmt.Errorf("too many options for %s and prompting has been disabled",
							project.Template().String(),
						)
					}

					selected, err := selection.Select[pack.Package]("select possible template", packages...)
					if err != nil {
						return err
					}

					pullPack = *selected
				}

				proc := paraprogress.NewProcess(
					fmt.Sprintf("pulling %s",
						unikraft.TypeNameVersion(pullPack),
					),
					func(ctx context.Context, w func(progress float64)) error {
						return pullPack.Pull(
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
		}

		// List the components
		components, err := project.Components(ctx)
		if err != nil {
			return err
		}
		for _, c := range components {
			queries = append(queries, []packmanager.QueryOption{
				packmanager.WithName(c.Name()),
				packmanager.WithVersion(c.Version()),
				packmanager.WithSource(c.Source()),
				packmanager.WithTypes(c.Type()),
				packmanager.WithRemote(opts.Update),
				packmanager.WithPlatform(opts.Platform),
				packmanager.WithArchitecture(opts.Architecture),
			})
		}

		if project.Runtime() != nil {
			queries = append(queries, []packmanager.QueryOption{
				packmanager.WithName(project.Runtime().Name()),
				packmanager.WithVersion(project.Runtime().Version()),
				packmanager.WithRemote(opts.Update),
				packmanager.WithPlatform(opts.Platform),
				packmanager.WithArchitecture(opts.Architecture),
			})
		}

		// Is this a list (space delimetered) of packages to pull?
	} else if len(args) > 0 {
		for _, arg := range args {
			queries = append(queries, []packmanager.QueryOption{
				packmanager.WithRemote(opts.Update),
				packmanager.WithName(arg),
				packmanager.WithArchitecture(opts.Architecture),
				packmanager.WithPlatform(opts.Platform),
				packmanager.WithKConfig(opts.KConfig),
			})
		}
	}

	if len(queries) == 0 {
		return fmt.Errorf("no components to pull")
	}

	var found []pack.Package
	var treeItems []*processtree.ProcessTreeItem

	for _, qopts := range queries {
		qopts := qopts
		query := packmanager.NewQuery(qopts...)
		treeItems = append(treeItems,
			processtree.NewProcessTreeItem(
				fmt.Sprintf("finding %s", query.String()),
				"",
				func(ctx context.Context) error {
					more, err := pm.Catalog(ctx, qopts...)
					if err != nil {
						log.G(ctx).
							WithField("format", pm.Format().String()).
							WithField("name", query.Name()).
							Warn(err)
						return nil
					}

					if len(more) == 0 {
						opts.Update = true
						return fmt.Errorf("could not find local reference for %s", query.String())
					}

					found = append(found, more...)

					return nil
				},
			),
		)
	}

	tree, err := processtree.NewProcessTree(
		ctx,
		[]processtree.ProcessTreeOption{
			processtree.IsParallel(parallel),
			processtree.WithRenderer(norender),
			processtree.WithFailFast(false),
			processtree.WithHideOnSuccess(true),
			processtree.WithHideError(!opts.Update),
		},
		treeItems...,
	)
	if err != nil {
		return err
	}

	// Try again with a remote search
	if err := tree.Start(); err != nil && (len(found) == 0 || !opts.Update) {
		treeItems = []*processtree.ProcessTreeItem{}
		for _, qopts := range queries {
			qopts := qopts
			query := packmanager.NewQuery(qopts...)
			treeItems = append(treeItems,
				processtree.NewProcessTreeItem(
					fmt.Sprintf("finding %s", query.String()),
					"",
					func(ctx context.Context) error {
						more, err := pm.Catalog(ctx, append(
							qopts,
							packmanager.WithRemote(true),
						)...)
						if err != nil {
							log.G(ctx).
								WithField("format", pm.Format().String()).
								WithField("name", query.Name()).
								Warn(err)
							return nil
						}

						if len(more) == 0 {
							return fmt.Errorf("could not find %s", query.String())
						}

						found = append(found, more...)

						return nil
					},
				),
			)
		}

		tree, err = processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(parallel),
				processtree.WithRenderer(norender),
				processtree.WithFailFast(false),
				processtree.WithHideOnSuccess(true),
			},
			treeItems...,
		)
		if err != nil {
			return err
		}

		if err := tree.Start(); err != nil {
			return err
		}
	}

	for _, p := range found {
		p := p
		processes = append(processes, paraprogress.NewProcess(
			fmt.Sprintf("pulling %s", p.String()),
			func(ctx context.Context, w func(progress float64)) error {
				return p.Pull(
					ctx,
					pack.WithPullProgressFunc(w),
					pack.WithPullWorkdir(opts.Output),
					pack.WithPullChecksum(!opts.NoChecksum),
					pack.WithPullCache(!opts.Update),
				)
			},
		))
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

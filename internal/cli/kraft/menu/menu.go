// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package menu

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"

	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/confirm"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"

	"kraftkit.sh/tui/processtree"
)

type MenuOptions struct {
	Architecture string `long:"arch" short:"m" usage:"Filter the creation of the build by architecture of known targets"`
	ForcePull    bool   `long:"force-pull" usage:"Force pulling components before opening the menu"`
	Frontend     string `long:"frontend" short:"f" usage:"Alternative frontend to use for the configuration editor" default:"menuconfig"`
	Kraftfile    string `long:"kraftfile" short:"K" usage:"Set an alternative path of the Kraftfile"`
	NoCache      bool   `long:"no-cache" usage:"Do not use the cache when pulling dependencies"`
	NoConfigure  bool   `long:"no-configure" usage:"Do not run Unikraft's configure step before building"`
	Platform     string `long:"plat" short:"p" usage:"Filter the creation of the build by platform of known targets"`
	Target       string `long:"target" short:"t" usage:"Build a particular known target"`

	project app.Application
	workdir string
}

func NewCmd() *cobra.Command {
	cmd, err := cmdfactory.New(&MenuOptions{}, cobra.Command{
		Short:   "Open's Unikraft configuration editor TUI",
		Use:     "menu [FLAGS] [DIR]",
		Aliases: []string{"menuconfig"},
		Args:    cmdfactory.MaxDirArgs(1),
		Long:    heredoc.Docf(`Open Unikraft's configuration editor TUI.`),
		Example: heredoc.Doc(`
			# Open configuration editor in the cwd project
			$ kraft menu

			# Open configuration editor for a project at a path
			$ kraft menu path/to/app`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *MenuOptions) Pre(cmd *cobra.Command, args []string) error {
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(cmd.Context())
	if err != nil {
		return err
	}

	cmd.SetContext(ctx)

	if len(args) == 0 {
		opts.workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		opts.workdir = args[0]
	}

	popts := []app.ProjectOption{
		app.WithProjectWorkdir(opts.workdir),
	}

	if len(opts.Kraftfile) > 0 {
		popts = append(popts, app.WithProjectKraftfile(opts.Kraftfile))
	} else {
		popts = append(popts, app.WithProjectDefaultKraftfiles())
	}

	// Initialize at least the configuration options for a project
	opts.project, err = app.NewProjectFromOptions(ctx, popts...)
	if err != nil && errors.Is(err, app.ErrNoKraftfile) {
		return fmt.Errorf("cannot build project directory without a Kraftfile")
	} else if err != nil {
		return fmt.Errorf("could not initialize project directory: %w", err)
	}

	opts.Platform = platform.PlatformByName(opts.Platform).String()

	return nil
}

func (opts *MenuOptions) pull(ctx context.Context, project app.Application, workdir string, norender bool, nameWidth int) error {
	var missingPacks []pack.Package
	var processes []*paraprogress.Process
	var searches []*processtree.ProcessTreeItem
	parallel := !config.G[config.KraftKit](ctx).NoParallel
	auths := config.G[config.KraftKit](ctx).Auth

	if _, err := opts.project.Components(ctx); err != nil && opts.project.Template().Name() != "" {
		var packages []pack.Package
		var pullPack pack.Package

		search := processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s",
				unikraft.TypeNameVersion(opts.project.Template()),
			), "",
			func(ctx context.Context) error {
				packages, err = packmanager.G(ctx).Catalog(ctx,
					packmanager.WithName(opts.project.Template().Name()),
					packmanager.WithTypes(unikraft.ComponentTypeApp),
					packmanager.WithVersion(opts.project.Template().Version()),
					packmanager.WithRemote(opts.NoCache),
					packmanager.WithAuthConfig(config.G[config.KraftKit](ctx).Auth),
				)
				if err != nil {
					return err
				}

				if len(packages) == 0 {
					return fmt.Errorf("could not find: %s",
						unikraft.TypeNameVersion(opts.project.Template()),
					)
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
			search,
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
					opts.project.Template().String(),
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
					pack.WithPullWorkdir(workdir),
					// pack.WithPullChecksum(!opts.NoChecksum),
					pack.WithPullCache(!opts.NoCache),
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
			paraprogress.WithNameWidth(nameWidth),
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return fmt.Errorf("could not pull all components: %v", err)
		}
	}

	if opts.project.Template() != nil && opts.project.Template().Name() != "" {
		templateWorkdir, err := unikraft.PlaceComponent(workdir, opts.project.Template().Type(), opts.project.Template().Name())
		if err != nil {
			return err
		}

		popts := []app.ProjectOption{
			app.WithProjectWorkdir(workdir),
		}

		if len(opts.Kraftfile) > 0 {
			popts = append(popts, app.WithProjectKraftfile(templateWorkdir))
		} else {
			popts = append(popts, app.WithProjectDefaultKraftfiles())
		}

		templateProject, err := app.NewProjectFromOptions(ctx, popts...)
		if err != nil {
			return err
		}

		opts.project, err = templateProject.MergeTemplate(ctx, project)
		if err != nil {
			return err
		}
	}

	// Overwrite template with user options
	components, err := opts.project.Components(ctx)
	if err != nil {
		return err
	}
	for _, component := range components {
		// Skip "finding" the component if path is the same as the source (which
		// means that the source code is already available as it is a directory on
		// disk.  In this scenario, the developer is likely hacking the particular
		// microlibrary/component.
		if component.Path() == component.Source() {
			continue
		}

		// Only continue to find and pull the component if it does not exist
		// locally or the user has requested to --force-pull.
		if stat, err := os.Stat(component.Path()); err == nil && stat.IsDir() && !opts.ForcePull {
			continue
		}

		component := component // loop closure
		auths := auths

		if f, err := os.Stat(component.Source()); err == nil && f.IsDir() {
			continue
		}

		searches = append(searches, processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s",
				unikraft.TypeNameVersion(component),
			), "",
			func(ctx context.Context) error {
				p, err := packmanager.G(ctx).Catalog(ctx,
					packmanager.WithName(component.Name()),
					packmanager.WithTypes(component.Type()),
					packmanager.WithVersion(component.Version()),
					packmanager.WithSource(component.Source()),
					packmanager.WithRemote(opts.NoCache),
					packmanager.WithAuthConfig(auths),
				)
				if err != nil {
					return err
				}

				if len(p) == 0 {
					return fmt.Errorf("could not find: %s",
						unikraft.TypeNameVersion(component),
					)
				} else if len(p) > 1 {
					return fmt.Errorf("too many options for %s",
						unikraft.TypeNameVersion(component),
					)
				}

				missingPacks = append(missingPacks, p...)
				return nil
			},
		))
	}

	if len(searches) > 0 {
		treemodel, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(parallel),
				processtree.WithRenderer(norender),
				processtree.WithFailFast(true),
			},
			searches...,
		)
		if err != nil {
			return err
		}

		if err := treemodel.Start(); err != nil {
			return fmt.Errorf("could not complete search: %v", err)
		}
	}

	if len(missingPacks) > 0 {
		for _, p := range missingPacks {
			p := p // loop closure
			auths := auths
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("pulling %s",
					unikraft.TypeNameVersion(p),
				),
				func(ctx context.Context, w func(progress float64)) error {
					return p.Pull(
						ctx,
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(workdir),
						// pack.WithPullChecksum(!opts.NoChecksum),
						pack.WithPullCache(!opts.NoCache),
						pack.WithPullAuthConfig(auths),
					)
				},
			))
		}

		paramodel, err := paraprogress.NewParaProgress(
			ctx,
			processes,
			paraprogress.IsParallel(parallel),
			paraprogress.WithRenderer(norender),
			paraprogress.WithFailFast(true),
			paraprogress.WithNameWidth(nameWidth),
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return fmt.Errorf("could not pull all components: %v", err)
		}
	}

	return nil
}

func (opts *MenuOptions) Run(ctx context.Context, _ []string) error {
	// Filter project targets by any provided CLI options
	selected := opts.project.Targets()

	selected = target.Filter(
		selected,
		opts.Architecture,
		opts.Platform,
		opts.Target,
	)

	if !config.G[config.KraftKit](ctx).NoPrompt {
		res, err := target.Select(selected)
		if err != nil {
			return err
		}
		selected = []target.Target{res}
	}

	if len(selected) == 0 {
		return fmt.Errorf("no targets selected to fetch")
	}

	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY
	nameWidth := -1

	// Calculate the width of the longest process name so that we can align the
	// two independent processtrees if we are using "render" mode (aka the fancy
	// mode is enabled).
	if !norender {
		// The longest word is "configuring" (which is 11 characters long), plus
		// additional space characters (2 characters), brackets (2 characters) the
		// name of the project and the target/plat string (which is variable in
		// length).
		for _, targ := range selected {
			if newLen := len(targ.Name()) + len(target.TargetPlatArchName(targ)) + 15; newLen > nameWidth {
				nameWidth = newLen
			}
		}
	}

	if err := opts.pull(ctx, opts.project, opts.workdir, norender, nameWidth); err != nil {
		return err
	}

	processes := []*paraprogress.Process{}

	for _, targ := range selected {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ

		if !opts.NoConfigure {
			var err error
			configure := true

			if opts.project.IsConfigured(targ) {
				configure, err = confirm.NewConfirm("project already configured, are you sure you want to rerun the configure step:")
				if err != nil {
					return err
				}
			}

			if configure {
				processes = append(processes, paraprogress.NewProcess(
					fmt.Sprintf("configuring %s (%s)", targ.Name(), target.TargetPlatArchName(targ)),
					func(ctx context.Context, w func(progress float64)) error {
						return opts.project.Configure(
							ctx,
							targ, // Target-specific options
							nil,  // No extra configuration options
							make.WithProgressFunc(w),
							make.WithSilent(true),
							make.WithExecOptions(
								exec.WithStdin(iostreams.G(ctx).In),
								exec.WithStdout(log.G(ctx).Writer()),
								exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
							),
						)
					},
				))
			} else {
				log.G(ctx).Info("skipping configure step")
				log.G(ctx).Info("to omit this prompt, use the '--no-configure' flag")
			}
		}
	}

	if len(processes) > 0 {
		paramodel, err := paraprogress.NewParaProgress(
			ctx,
			processes,
			// Disable parallelization as:
			//  - The first process may be pulling the container image, which is
			//    necessary for the subsequent build steps;
			//  - The Unikraft build system can re-use compiled files from previous
			//    compilations (if the architecture does not change).
			paraprogress.IsParallel(false),
			paraprogress.WithRenderer(norender),
			paraprogress.WithFailFast(true),
		)
		if err != nil {
			return err
		}

		err = paramodel.Start()
		if err != nil {
			return fmt.Errorf("could not configure all targets: %v", err)
		}
	}

	return opts.project.Make(
		ctx,
		selected[0],
		make.WithTarget(opts.Frontend),
		make.WithExecOptions(
			exec.WithStdout(iostreams.G(ctx).Out),
			exec.WithStdin(iostreams.G(ctx).In),
		),
	)
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

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
	"kraftkit.sh/machine/platform"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"

	"kraftkit.sh/tui/processtree"
)

type Build struct {
	All          bool   `long:"all" usage:"Build all targets"`
	Architecture string `long:"arch" short:"m" usage:"Filter the creation of the build by architecture of known targets"`
	DotConfig    string `long:"config" short:"c" usage:"Override the path to the KConfig .config file"`
	Jobs         int    `long:"jobs" short:"j" usage:"Allow N jobs at once"`
	KernelDbg    bool   `long:"dbg" usage:"Build the debuggable (symbolic) kernel image instead of the stripped image"`
	Kraftfile    string `long:"kraftfile" usage:"Set an alternative path of the Kraftfile"`
	NoCache      bool   `long:"no-cache" short:"F" usage:"Force a rebuild even if existing intermediate artifacts already exist"`
	NoConfigure  bool   `long:"no-configure" usage:"Do not run Unikraft's configure step before building"`
	NoFast       bool   `long:"no-fast" usage:"Do not use maximum parallelization when performing the build"`
	NoFetch      bool   `long:"no-fetch" usage:"Do not run Unikraft's fetch step before building"`
	NoPull       bool   `long:"no-pull" usage:"Do not pull packages before invoking Unikraft's build system"`
	NoUpdate     bool   `long:"no-update" usage:"Do not update package index before running the build"`
	Platform     string `long:"plat" short:"p" usage:"Filter the creation of the build by platform of known targets"`
	SaveBuildLog string `long:"build-log" usage:"Use the specified file to save the output from the build"`
	Target       string `long:"target" short:"t" usage:"Build a particular known target"`

	project app.Application
	workdir string
}

func New() *cobra.Command {
	cmd, err := cmdfactory.New(&Build{}, cobra.Command{
		Short: "Configure and build Unikraft unikernels",
		Use:   "build [FLAGS] [SUBCOMMAND|DIR]",
		Args:  cmdfactory.MaxDirArgs(1),
		Long: heredoc.Docf(`
			Build a Unikraft unikernel.

			The default behaviour of %[1]skraft build%[1]s is to build a project.  Given no
			arguments, you will be guided through interactive mode.
		`, "`"),
		Example: heredoc.Doc(`
			# Build the current project (cwd)
			$ kraft build

			# Build path to a Unikraft project
			$ kraft build path/to/app`),
		Annotations: map[string]string{
			cmdfactory.AnnotationHelpGroup: "build",
		},
	})
	if err != nil {
		panic(err)
	}

	return cmd
}

func (opts *Build) Pre(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	pm, err := packmanager.NewUmbrellaManager(ctx)
	if err != nil {
		return err
	}

	cmd.SetContext(packmanager.WithPackageManager(ctx, pm))

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

func (opts *Build) pull(ctx context.Context, project app.Application, workdir string, norender bool, nameWidth int) error {
	var missingPacks []pack.Package
	var processes []*paraprogress.Process
	var searches []*processtree.ProcessTreeItem
	parallel := !config.G[config.KraftKit](ctx).NoParallel
	auths := config.G[config.KraftKit](ctx).Auth

	if _, err := opts.project.Components(ctx); err != nil && opts.project.Template().Name() != "" {
		var packages []pack.Package
		search := processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s",
				unikraft.TypeNameVersion(opts.project.Template()),
			), "",
			func(ctx context.Context) error {
				packages, err = packmanager.G(ctx).Catalog(ctx,
					packmanager.WithName(opts.project.Template().Name()),
					packmanager.WithTypes(unikraft.ComponentTypeApp),
					packmanager.WithVersion(opts.project.Template().Version()),
					packmanager.WithCache(!opts.NoCache),
					packmanager.WithAuthConfig(config.G[config.KraftKit](ctx).Auth),
				)
				if err != nil {
					return err
				}

				if len(packages) == 0 {
					return fmt.Errorf("could not find: %s",
						unikraft.TypeNameVersion(opts.project.Template()),
					)
				} else if len(packages) > 1 {
					return fmt.Errorf("too many options for %s",
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

		proc := paraprogress.NewProcess(
			fmt.Sprintf("pulling %s",
				unikraft.TypeNameVersion(packages[0]),
			),
			func(ctx context.Context, w func(progress float64)) error {
				return packages[0].Pull(
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

	if opts.project.Template().Name() != "" {
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
					packmanager.WithCache(!opts.NoCache),
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

func (opts *Build) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Filter project targets by any provided CLI options
	selected := opts.project.Targets()
	if !opts.All {
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
	}

	if len(selected) == 0 {
		return fmt.Errorf("no targets selected to build")
	}

	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	if !opts.NoUpdate {
		model, err := processtree.NewProcessTree(
			ctx,
			[]processtree.ProcessTreeOption{
				processtree.IsParallel(!config.G[config.KraftKit](ctx).NoParallel),
				processtree.WithRenderer(norender),
			},
			[]*processtree.ProcessTreeItem{
				processtree.NewProcessTreeItem(
					"updating package index",
					"",
					func(ctx context.Context) error {
						return packmanager.G(ctx).Update(ctx)
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
	}

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

	if !opts.NoPull {
		if err := opts.pull(ctx, opts.project, opts.workdir, norender, nameWidth); err != nil {
			return err
		}
	}

	processes := []*paraprogress.Process{}

	var mopts []make.MakeOption
	if opts.Jobs > 0 {
		mopts = append(mopts, make.WithJobs(opts.Jobs))
	} else {
		mopts = append(mopts, make.WithMaxJobs(!opts.NoFast && !config.G[config.KraftKit](ctx).NoParallel))
	}

	for _, targ := range selected {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ
		if !opts.NoConfigure {
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
		}

		processes = append(processes, paraprogress.NewProcess(
			fmt.Sprintf("building %s (%s)", targ.Name(), target.TargetPlatArchName(targ)),
			func(ctx context.Context, w func(progress float64)) error {
				return opts.project.Build(
					ctx,
					targ, // Target-specific options
					app.WithBuildProgressFunc(w),
					app.WithBuildMakeOptions(append(mopts,
						make.WithExecOptions(
							exec.WithStdout(log.G(ctx).Writer()),
							exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
						),
					)...),
					app.WithBuildLogFile(opts.SaveBuildLog),
				)
			},
		))
	}

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

	return paramodel.Start()
}

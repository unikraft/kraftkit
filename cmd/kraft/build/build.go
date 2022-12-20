// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package build

import (
	"context"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
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
	Architecture string `long:"arch" short:"m" usage:"Filter the creation of the build by architecture of known targets"`
	DotConfig    string `long:"config" short:"c" usage:"Override the path to the KConfig .config file"`
	Fast         bool   `long:"fast" usage:"Use maximum parallization when performing the build"`
	Jobs         int    `long:"jobs" short:"j" usage:"Allow N jobs at once"`
	KernelDbg    bool   `long:"dbg" usage:"Build the debuggable (symbolic) kernel image instead of the stripped image"`
	NoCache      bool   `long:"no-cache" short:"F" usage:"Force a rebuild even if existing intermediate artifacts already exist"`
	NoConfigure  bool   `long:"no-configure" usage:"Do not run Unikraft's configure step before building"`
	NoFetch      bool   `long:"no-fetch" usage:"Do not run Unikraft's fetch step before building"`
	NoPrepare    bool   `long:"no-prepare" usage:"Do not run Unikraft's prepare step before building"`
	Platform     string `long:"plat" short:"p" usage:"Filter the creation of the build by platform of known targets"`
	SaveBuildLog string `long:"build-log" usage:"Use the specified file to save the output from the build"`
	Target       string `long:"target" short:"t" usage:"Build a particular known target"`
}

func New() *cobra.Command {
	return cmdfactory.New(&Build{}, cobra.Command{
		Short: "Configure and build Unikraft unikernels ",
		Use:   "build [FLAGS] [SUBCOMMAND|DIR]",
		Args:  cmdfactory.MaxDirArgs(1),
		Long: heredoc.Docf(`
			Configure and build Unikraft unikernels.

			The default behaviour of %[1]skraft build%[1]s is to build a project.  Given no
			arguments, you will be guided through interactive mode.
		`, "`"),
		Example: heredoc.Doc(`
			# Build the current project (cwd)
			$ kraft build

			# Build path to a Unikraft project
			$ kraft build path/to/app`),
		Annotations: map[string]string{
			"help:group": "build",
		},
	})
}

func (opts *Build) Run(cmd *cobra.Command, args []string) error {
	var err error
	var workdir string

	if (len(opts.Architecture) > 0 || len(opts.Platform) > 0) && len(opts.Target) > 0 {
		return fmt.Errorf("the `--arch` and `--plat` options are not supported in addition to `--target`")
	}

	if len(args) == 0 {
		workdir, err = os.Getwd()
		if err != nil {
			return err
		}
	} else {
		workdir = args[0]
	}

	ctx := cmd.Context()

	// Initialize at least the configuration options for a project
	projectOpts, err := app.NewProjectOptions(
		nil,
		app.WithProjectWorkdir(workdir),
		app.WithProjectDefaultConfigPath(),
		app.WithProjectResolvedPaths(true),
		app.WithProjectDotConfig(false),
	)
	if err != nil {
		return err
	}

	if !app.IsWorkdirInitialized(workdir) {
		return fmt.Errorf("cannot build uninitialized project! start with: ukbuild init")
	}

	// Interpret the project directory
	project, err := app.NewProjectFromOptions(projectOpts)
	if err != nil {
		return err
	}

	parallel := !config.G(ctx).NoParallel
	norender := log.LoggerTypeFromString(config.G(ctx).Log.Type) != log.FANCY

	var missingPacks []pack.Package
	var processes []*paraprogress.Process
	var searches []*processtree.ProcessTreeItem

	_, err = project.Components()
	if err != nil && project.Template().Name() != "" {
		var packages []pack.Package
		search := processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s/%s:%s...", project.Template().Type(), project.Template().Name(), project.Template().Version()), "",
			func(ctx context.Context) error {
				packages, err = packmanager.G(ctx).Catalog(ctx, packmanager.CatalogQuery{
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
			search,
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
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return fmt.Errorf("could not pull all components: %v", err)
		}
	}

	if project.Template().Name() != "" {
		templateWorkdir, err := unikraft.PlaceComponent(workdir, project.Template().Type(), project.Template().Name())
		if err != nil {
			return err
		}

		templateOps, err := app.NewProjectOptions(
			nil,
			app.WithProjectWorkdir(templateWorkdir),
			app.WithProjectDefaultConfigPath(),
			app.WithProjectResolvedPaths(true),
			app.WithProjectDotConfig(false),
		)
		if err != nil {
			return err
		}

		templateProject, err := app.NewProjectFromOptions(templateOps)
		if err != nil {
			return err
		}

		project = templateProject.MergeTemplate(project)
	}

	// Overwrite template with user options
	components, err := project.Components()
	if err != nil {
		return err
	}
	for _, component := range components {
		component := component // loop closure

		searches = append(searches, processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s/%s:%s...", component.Type(), component.Component().Name, component.Component().Version), "",
			func(ctx context.Context) error {
				p, err := packmanager.G(ctx).Catalog(ctx, packmanager.CatalogQuery{
					Name: component.Name(),
					Types: []unikraft.ComponentType{
						component.Type(),
					},
					Version: component.Version(),
					NoCache: opts.NoCache,
				})
				if err != nil {
					return err
				}

				if len(p) == 0 {
					return fmt.Errorf("could not find: %s", component.Component().Name)
				} else if len(p) > 1 {
					return fmt.Errorf("too many options for %s", component.Component().Name)
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
			if p.Options() == nil {
				return fmt.Errorf("unexpected error occurred please try again")
			}
			p := p // loop closure
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("pulling %s", p.Options().TypeNameVersion()),
				func(ctx context.Context, w func(progress float64)) error {
					return p.Pull(
						ctx,
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(workdir),
						// pack.WithPullChecksum(!opts.NoChecksum),
						pack.WithPullCache(!opts.NoCache),
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
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return fmt.Errorf("could not pull all components: %v", err)
		}
	}

	processes = []*paraprogress.Process{} // reset

	var selected target.Targets
	targets, err := project.Targets()
	if err != nil {
		return err
	}

	// Filter the targets by CLI selection
	for _, targ := range targets {
		switch true {
		case
			// If no arguments are supplied
			len(opts.Target) == 0 &&
				len(opts.Architecture) == 0 &&
				len(opts.Platform) == 0,

			// If the --target flag is supplied and the target name match
			len(opts.Target) > 0 &&
				targ.Name() == opts.Target,

			// If only the --arch flag is supplied and the target's arch matches
			len(opts.Architecture) > 0 &&
				len(opts.Platform) == 0 &&
				targ.Architecture.Name() == opts.Architecture,

			// If only the --plat flag is supplied and the target's platform matches
			len(opts.Platform) > 0 &&
				len(opts.Architecture) == 0 &&
				targ.Platform.Name() == opts.Platform,

			// If both the --arch and --plat flag are supplied and match the target
			len(opts.Platform) > 0 &&
				len(opts.Architecture) > 0 &&
				targ.Architecture.Name() == opts.Architecture &&
				targ.Platform.Name() == opts.Platform:

			selected = append(selected, targ)

		default:
			continue
		}
	}

	if len(selected) == 0 {
		log.G(ctx).Info("no selected to build")
		return nil
	}

	var mopts []make.MakeOption
	if opts.Jobs > 0 {
		mopts = append(mopts, make.WithJobs(opts.Jobs))
	} else {
		mopts = append(mopts, make.WithMaxJobs(opts.Fast))
	}

	for _, targ := range selected {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ
		if !opts.NoConfigure {
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("configuring %s (%s)", targ.Name(), targ.ArchPlatString()),
				func(ctx context.Context, w func(progress float64)) error {
					return project.DefConfig(
						ctx,
						&targ, // Target-specific options
						nil,   // No extra configuration options
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

		if !opts.NoPrepare {
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("preparing %s (%s)", targ.Name(), targ.ArchPlatString()),
				func(ctx context.Context, w func(progress float64)) error {
					return project.Prepare(
						ctx,
						&targ, // Target-specific options
						append(
							mopts,
							make.WithProgressFunc(w),
							make.WithExecOptions(
								exec.WithStdout(log.G(ctx).Writer()),
								exec.WithStderr(log.G(ctx).WriterLevel(logrus.ErrorLevel)),
							),
						)...,
					)
				},
			))
		}

		processes = append(processes, paraprogress.NewProcess(
			fmt.Sprintf("building %s (%s)", targ.Name(), targ.ArchPlatString()),
			func(ctx context.Context, w func(progress float64)) error {
				return project.Build(
					ctx,
					&targ, // Target-specific options
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

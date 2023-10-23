// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/target"
)

type builderKraftfileUnikraft struct{}

// String implements fmt.Stringer.
func (build *builderKraftfileUnikraft) String() string {
	return "unikraft"
}

// Buildable implements builder.
func (build *builderKraftfileUnikraft) Buildable(ctx context.Context, opts *Build, args ...string) (bool, error) {
	if opts.project.Unikraft(ctx) == nil && opts.project.Template() == nil {
		return false, fmt.Errorf("cannot build without unikraft core specification")
	}

	if opts.Rootfs == "" {
		opts.Rootfs = opts.project.Rootfs()
	}

	return true, nil
}

func (build *builderKraftfileUnikraft) pull(ctx context.Context, opts *Build, norender bool, nameWidth int) error {
	var missingPacks []pack.Package
	var processes []*paraprogress.Process
	var searches []*processtree.ProcessTreeItem
	parallel := !config.G[config.KraftKit](ctx).NoParallel
	auths := config.G[config.KraftKit](ctx).Auth

	// FIXME: This is a temporary workaround for incorporating multiple processes in
	// a command. After calling processtree the original output writer is lost
	// so writing will no longer work. To fix this we temporarily save it
	// beforehand.

	// A proper fix would ensure in the tui package that this writer is
	// preserved. Thankfully, this is the only place where it manifests right
	// now.
	oldOut := iostreams.G(ctx).Out
	defer func() {
		iostreams.G(ctx).Out = oldOut
	}()

	if template := opts.project.Template(); template != nil {
		if stat, err := os.Stat(template.Path()); err != nil || !stat.IsDir() || opts.ForcePull {
			var templatePack pack.Package

			treemodel, err := processtree.NewProcessTree(
				ctx,
				[]processtree.ProcessTreeOption{
					processtree.IsParallel(parallel),
					processtree.WithRenderer(norender),
					processtree.WithFailFast(true),
				},
				processtree.NewProcessTreeItem(
					fmt.Sprintf("finding %s",
						unikraft.TypeNameVersion(template),
					), "",
					func(ctx context.Context) error {
						p, err := packmanager.G(ctx).Catalog(ctx,
							packmanager.WithName(template.Name()),
							packmanager.WithTypes(template.Type()),
							packmanager.WithVersion(template.Version()),
							packmanager.WithSource(template.Source()),
							packmanager.WithUpdate(opts.NoCache),
							packmanager.WithAuthConfig(auths),
						)
						if err != nil {
							return err
						}

						if len(p) == 0 {
							return fmt.Errorf("could not find: %s",
								unikraft.TypeNameVersion(template),
							)
						} else if len(p) > 1 {
							return fmt.Errorf("too many options for %s",
								unikraft.TypeNameVersion(template),
							)
						}

						templatePack = p[0]
						return nil
					},
				),
			)
			if err != nil {
				return err
			}

			if err := treemodel.Start(); err != nil {
				return fmt.Errorf("could not complete search: %v", err)
			}

			paramodel, err := paraprogress.NewParaProgress(
				ctx,
				[]*paraprogress.Process{
					paraprogress.NewProcess(
						fmt.Sprintf("pulling %s",
							unikraft.TypeNameVersion(template),
						),
						func(ctx context.Context, w func(progress float64)) error {
							return templatePack.Pull(
								ctx,
								pack.WithPullProgressFunc(w),
								pack.WithPullWorkdir(opts.workdir),
								// pack.WithPullChecksum(!opts.NoChecksum),
								pack.WithPullCache(!opts.NoCache),
								pack.WithPullAuthConfig(auths),
							)
						},
					),
				},
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

		templateProject, err := app.NewProjectFromOptions(ctx,
			app.WithProjectWorkdir(template.Path()),
			app.WithProjectDefaultKraftfiles(),
		)
		if err != nil {
			return err
		}

		// Overwrite template with user options
		opts.project, err = opts.project.MergeTemplate(ctx, templateProject)
		if err != nil {
			return err
		}
	}

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
					packmanager.WithUpdate(opts.NoCache),
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
						pack.WithPullWorkdir(opts.workdir),
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

func (build *builderKraftfileUnikraft) Build(ctx context.Context, opts *Build, targets []target.Target, args ...string) error {
	var processes []*paraprogress.Process

	nameWidth := -1
	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	// Calculate the width of the longest process name so that we can align the
	// two independent processtrees if we are using "render" mode (aka the fancy
	// mode is enabled).
	if !norender {
		// The longest word is "configuring" (which is 11 characters long), plus
		// additional space characters (2 characters), brackets (2 characters) the
		// name of the project and the target/plat string (which is variable in
		// length).
		for _, targ := range targets {
			if newLen := len(targ.Name()) + len(target.TargetPlatArchName(targ)) + 15; newLen > nameWidth {
				nameWidth = newLen
			}
		}

		components, err := opts.project.Components(ctx)
		if err != nil {
			return fmt.Errorf("could not get list of components: %w", err)
		}

		// The longest word is "pulling" (which is 7 characters long),plus
		// additional space characters (1 character).
		for _, component := range components {
			if newLen := len(unikraft.TypeNameVersion(component)) + 8; newLen > nameWidth {
				nameWidth = newLen
			}
		}
	}

	if opts.ForcePull || !opts.NoUpdate {
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

	if err := build.pull(ctx, opts, norender, nameWidth); err != nil {
		return err
	}

	var mopts []make.MakeOption
	if opts.Jobs > 0 {
		mopts = append(mopts, make.WithJobs(opts.Jobs))
	} else {
		mopts = append(mopts, make.WithMaxJobs(!opts.NoFast && !config.G[config.KraftKit](ctx).NoParallel))
	}

	for _, targ := range targets {
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
				err := opts.project.Build(
					ctx,
					targ, // Target-specific options
					app.WithBuildProgressFunc(w),
					app.WithBuildMakeOptions(append(mopts,
						make.WithExecOptions(
							exec.WithStdout(log.G(ctx).Writer()),
							exec.WithStderr(log.G(ctx).WriterLevel(logrus.WarnLevel)),
							// exec.WithOSEnv(true),
						),
					)...),
					app.WithBuildLogFile(opts.SaveBuildLog),
				)
				if err != nil {
					return fmt.Errorf("build failed: %w", err)
				}

				return nil
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
		paraprogress.WithNameWidth(nameWidth),
	)
	if err != nil {
		return err
	}

	return paramodel.Start()
}

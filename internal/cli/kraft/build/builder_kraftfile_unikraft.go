// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"fmt"
	"os"
	plainexec "os/exec"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/confirm"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/tui/selection"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
	"kraftkit.sh/unikraft/export/v0/posixenviron"
	"kraftkit.sh/unikraft/target"
)

type builderKraftfileUnikraft struct {
	nameWidth int
}

// String implements fmt.Stringer.
func (build *builderKraftfileUnikraft) String() string {
	return "kraftfile-unikraft"
}

// Buildable implements builder.
func (build *builderKraftfileUnikraft) Buildable(ctx context.Context, opts *BuildOptions, args ...string) (bool, error) {
	if opts.project.Unikraft(ctx) == nil && opts.project.Template() == nil {
		return false, fmt.Errorf("cannot build without unikraft core specification")
	}

	if opts.Rootfs == "" {
		opts.Rootfs = opts.project.Rootfs()
	}

	return true, nil
}

// Calculate lines of code in a kernel image.
// Requires objdump to be installed and debug symbols to be enabled.
func linesOfCode(ctx context.Context, opts *BuildOptions) (int64, error) {
	objdumpPath, err := plainexec.LookPath("objdump")
	if err != nil {
		log.G(ctx).Warn("objdump not found, skipping LoC statistics")
		return 0, nil
	}
	cmd := plainexec.CommandContext(ctx, objdumpPath, "-dl", (*opts.Target).KernelDbg())
	cmd.Stderr = log.G(ctx).WriterLevel(logrus.DebugLevel)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("running objdump: %w", err)
	}

	uniqueLines := map[string]bool{}
	filterRegex1 := regexp.MustCompile(`^/.*$`)
	filterRegex2 := regexp.MustCompile(`^/[/*].*$`)
	filterRegex3 := regexp.MustCompile(` [(]discriminator [0-9]+[)]`)
	for _, line := range strings.Split(string(out), "\n") {
		if filterRegex1.FindString(line) != "" &&
			filterRegex2.FindString(line) == "" {
			uniqueLines[filterRegex3.ReplaceAllString(line, "")] = true
		}
	}

	return int64(len(uniqueLines)), nil
}

func (build *builderKraftfileUnikraft) pull(ctx context.Context, opts *BuildOptions, norender bool, nameWidth int) error {
	var missingPacks []pack.Package
	var processes []*paraprogress.Process
	var searches []*processtree.ProcessTreeItem
	parallel := !config.G[config.KraftKit](ctx).NoParallel
	auths := config.G[config.KraftKit](ctx).Auth

	if template := opts.project.Template(); template != nil {
		if stat, err := os.Stat(template.Path()); err != nil || !stat.IsDir() || opts.ForcePull {
			var templatePack pack.Package
			var packs []pack.Package

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
							packmanager.WithRemote(opts.NoCache),
							packmanager.WithAuthConfig(auths),
						)
						if err != nil {
							return err
						}

						if len(p) == 0 {
							return fmt.Errorf("could not find: %s",
								unikraft.TypeNameVersion(template),
							)
						}

						packs = append(packs, p...)

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

			if len(packs) == 1 {
				templatePack = packs[0]
			} else if len(packs) > 1 {
				if config.G[config.KraftKit](ctx).NoPrompt {
					for _, p := range packs {
						log.G(ctx).
							WithField("template", p.String()).
							Warn("possible")
					}

					return fmt.Errorf("too many options for %s and prompting has been disabled",
						unikraft.TypeNameVersion(template),
					)
				}

				selected, err := selection.Select[pack.Package]("select possible template", packs...)
				if err != nil {
					return err
				}

				templatePack = *selected
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
								pack.WithPullWorkdir(opts.Workdir),
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

	components, err := opts.project.Components(ctx, *opts.Target)
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
						pack.WithPullWorkdir(opts.Workdir),
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
			paraprogress.WithNameWidth(build.nameWidth),
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

func (build *builderKraftfileUnikraft) Prepare(ctx context.Context, opts *BuildOptions, args ...string) error {
	build.nameWidth = -1
	norender := log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY

	if opts.Target == nil {
		// Filter project targets by any provided CLI options
		selected := opts.project.Targets()
		if len(selected) == 0 {
			return fmt.Errorf("no targets to build")
		}
		if !opts.All {
			selected = target.Filter(
				selected,
				opts.Architecture,
				opts.Platform,
				opts.TargetName,
			)

			if !config.G[config.KraftKit](ctx).NoPrompt && len(selected) > 1 {
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

		opts.Target = &selected[0]
	}

	// Calculate the width of the longest process name so that we can align the
	// two independent processtrees if we are using "render" mode (aka the fancy
	// mode is enabled).
	if !norender {
		// The longest word is "configuring" (which is 11 characters long), plus
		// additional space characters (2 characters), brackets (2 characters) the
		// name of the project and the target/plat string (which is variable in
		// length).
		if newLen := len((*opts.Target).Name()) + len(target.TargetPlatArchName(*opts.Target)) + 15; newLen > build.nameWidth {
			build.nameWidth = newLen
		}

		components, err := opts.project.Components(ctx, *opts.Target)
		if err != nil {
			return fmt.Errorf("could not get list of components: %w", err)
		}

		// The longest word is "pulling" (which is 7 characters long),plus
		// additional space characters (1 character).
		for _, component := range components {
			if newLen := len(unikraft.TypeNameVersion(component)) + 8; newLen > build.nameWidth {
				build.nameWidth = newLen
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
					"updating index",
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

	return build.pull(ctx, opts, norender, build.nameWidth)
}

func (build *builderKraftfileUnikraft) Build(ctx context.Context, opts *BuildOptions, args ...string) error {
	var processes []*paraprogress.Process
	var mopts []make.MakeOption
	if opts.Jobs > 0 {
		mopts = append(mopts, make.WithJobs(opts.Jobs))
	} else {
		mopts = append(mopts, make.WithMaxJobs(!opts.NoFast && !config.G[config.KraftKit](ctx).NoParallel))
	}

	allEnvs := map[string]string{}
	for k, v := range opts.project.Env() {
		allEnvs[k] = v

		if v == "" {
			allEnvs[k] = os.Getenv(k)
		}
	}

	for _, env := range opts.Env {
		if strings.ContainsRune(env, '=') {
			parts := strings.SplitN(env, "=", 2)
			allEnvs[parts[0]] = parts[1]
		} else {
			allEnvs[env] = os.Getenv(env)
		}
	}

	// There might already be environment variables in the project Kconfig,
	// so we need to be careful with indexing
	counter := 1
	envKconfig := kconfig.KeyValueMap{}
	for k, v := range allEnvs {
		for counter <= posixenviron.DefaultCompiledInLimit {
			val, found := opts.project.KConfig().Get(fmt.Sprintf("LIBPOSIX_ENVIRON_ENVP%d", counter))
			if !found || val.Value == "" {
				break
			}
			counter += 1
		}

		if counter > posixenviron.DefaultCompiledInLimit {
			log.G(ctx).Warnf("cannot compile in more than %d environment variables, skipping %s", posixenviron.DefaultCompiledInLimit, k)
			continue
		}

		envKconfig.Set(fmt.Sprintf("CONFIG_LIBPOSIX_ENVIRON_ENVP%d", counter), fmt.Sprintf("%s=%s", k, v))
		counter++
	}

	if !opts.NoConfigure {
		var err error
		configure := true

		if opts.project.IsConfigured(*opts.Target) {
			configure, err = confirm.NewConfirm("project already configured, are you sure you want to rerun the configure step:")
			if err != nil {
				return err
			}
		}

		if configure {
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("configuring %s (%s)", (*opts.Target).Name(), target.TargetPlatArchName(*opts.Target)),
				func(ctx context.Context, w func(progress float64)) error {
					return opts.project.Configure(
						ctx,
						*opts.Target, // Target-specific options
						envKconfig,   // Extra Kconfigs for compiled in environment variables
						make.WithProgressFunc(w),
						make.WithSilent(true),
						make.WithExecOptions(
							exec.WithStdin(iostreams.G(ctx).In),
							exec.WithStdout(log.G(ctx).Writer()),
							exec.WithStderr(log.G(ctx).WriterLevel(logrus.WarnLevel)),
						),
					)
				},
			))
		} else {
			log.G(ctx).Info("skipping configure step")
			log.G(ctx).Info("to omit this prompt, use the '--no-configure' flag")
		}
	}

	processes = append(processes, paraprogress.NewProcess(
		fmt.Sprintf("building %s (%s)", (*opts.Target).Name(), target.TargetPlatArchName(*opts.Target)),
		func(ctx context.Context, w func(progress float64)) error {
			err := opts.project.Build(
				ctx,
				*opts.Target, // Target-specific options
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

	paramodel, err := paraprogress.NewParaProgress(
		ctx,
		processes,
		// Disable parallelization as:
		//  - The first process may be pulling the container image, which is
		//    necessary for the subsequent build steps;
		//  - The Unikraft build system can re-use compiled files from previous
		//    compilations (if the architecture does not change).
		paraprogress.IsParallel(false),
		paraprogress.WithRenderer(log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY),
		paraprogress.WithFailFast(true),
		paraprogress.WithNameWidth(build.nameWidth),
	)
	if err != nil {
		return err
	}

	return paramodel.Start()
}

func (build *builderKraftfileUnikraft) Statistics(ctx context.Context, opts *BuildOptions, args ...string) error {
	var processes []*paraprogress.Process

	processes = append(processes, paraprogress.NewProcess(
		fmt.Sprintf("statistics %s (%s)", (*opts.Target).Name(), target.TargetPlatArchName(*opts.Target)),
		func(ctx context.Context, w func(progress float64)) error {
			lines, err := linesOfCode(ctx, opts)
			if lines > 1 {
				opts.statistics["lines of code"] = fmt.Sprintf("%d", lines)
			}
			return err
		},
	))

	paramodel, err := paraprogress.NewParaProgress(
		ctx,
		processes,
		paraprogress.IsParallel(false),
		paraprogress.WithRenderer(log.LoggerTypeFromString(config.G[config.KraftKit](ctx).Log.Type) != log.FANCY),
		paraprogress.WithFailFast(true),
		paraprogress.WithNameWidth(build.nameWidth),
	)
	if err != nil {
		return err
	}

	return paramodel.Start()
}

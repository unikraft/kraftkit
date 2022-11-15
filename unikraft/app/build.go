// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//          Cezar Craciunoiu <cezar@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package app

import (
	"fmt"
	"os"

	"kraftkit.sh/exec"
	"kraftkit.sh/internal/logger"
	"kraftkit.sh/log"

	kmake "kraftkit.sh/make"

	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"

	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"

	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type CommandBuildArgs struct {
	NoCache      bool   `usage:"Do not use cache when building the image" default:"false"`
	Architecture string `usage:"The machine architecture of the resulting image"`
	Platform     string `usage:"The platform to run the resulting image on"`
	DotConfig    string `usage:"Override the path to the Kconfig file"`
	Target       string `usage:"Build a particular target"`
	KernelDbg    bool   `usage:"Build the image with debugging symbols in" default:"false"`
	Fast         bool   `usage:"Use maximum parallization when performing the build" default:"false"`
	Jobs         int    `usage:"Allow N jobs at once" default:"0"`
	NoSyncConfig bool   `usage:"Do not synchronize Unikraft's configuration before building" default:"false"`
	NoPrepare    bool   `usage:"Do not run Unikraft's prepare step before building" default:"false"`
	NoFetch      bool   `usage:"Do not run Unikraft's fetch step before building" default:"false"`
	NoPull       bool   `usage:"Do not run Unikraft's pull step before building" default:"false"`
	NoConfigure  bool   `usage:"Do not run Unikraft's configure step before building" default:"false"`
	SaveBuildLog string `usage:"Use the specified file to save the output from the build"`
}

func (copts *CommandOptions) Clean() error {
	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := NewProjectOptions(
		nil,
		WithLogger(plog),
		WithWorkingDirectory(copts.Workdir),
		WithDefaultConfigPath(),
		WithPackageManager(&pm),
		WithResolvedPaths(true),
		WithDotConfig(false),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Clean(
		kmake.WithExecOptions(
			exec.WithStdin(copts.IO.In),
			exec.WithStdout(copts.IO.Out),
			exec.WithStderr(copts.IO.ErrOut),
		),
	)
}

func (copts *CommandOptions) Configure() error {
	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := NewProjectOptions(
		nil,
		WithLogger(plog),
		WithWorkingDirectory(copts.Workdir),
		WithDefaultConfigPath(),
		WithPackageManager(&pm),
		WithResolvedPaths(true),
		WithDotConfig(false),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Configure(
		kmake.WithExecOptions(
			exec.WithStdin(copts.IO.In),
		),
	)
}

func (copts *CommandOptions) Fetch() error {
	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := NewProjectOptions(
		nil,
		WithLogger(plog),
		WithWorkingDirectory(copts.Workdir),
		WithDefaultConfigPath(),
		WithPackageManager(&pm),
		WithResolvedPaths(true),
		WithDotConfig(false),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Fetch(
		kmake.WithExecOptions(
			exec.WithStdin(copts.IO.In),
		),
	)
}

func (copts *CommandOptions) MenuConfig() error {
	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := NewProjectOptions(
		nil,
		WithLogger(plog),
		WithWorkingDirectory(copts.Workdir),
		WithDefaultConfigPath(),
		WithPackageManager(&pm),
		WithResolvedPaths(true),
		WithDotConfig(false),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Make(
		kmake.WithExecOptions(
			exec.WithStdin(copts.IO.In),
			exec.WithStdout(copts.IO.Out),
		),
		kmake.WithTarget("menuconfig"),
	)
}

func (copts *CommandOptions) Prepare() error {
	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := NewProjectOptions(
		nil,
		WithLogger(plog),
		WithWorkingDirectory(copts.Workdir),
		WithDefaultConfigPath(),
		WithPackageManager(&pm),
		WithResolvedPaths(true),
		WithDotConfig(false),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Prepare(
		kmake.WithExecOptions(
			exec.WithStdin(copts.IO.In),
		),
	)
}

func (copts *CommandOptions) Properclean() error {
	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := NewProjectOptions(
		nil,
		WithLogger(plog),
		WithWorkingDirectory(copts.Workdir),
		WithDefaultConfigPath(),
		WithPackageManager(&pm),
		WithResolvedPaths(true),
		WithDotConfig(false),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Properclean(
		kmake.WithExecOptions(
			exec.WithStdin(copts.IO.In),
			exec.WithStdout(copts.IO.Out),
			exec.WithStderr(copts.IO.ErrOut),
		),
	)
}

func (copts *CommandOptions) Set(confOpts []string) error {
	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Check if dotconfig exists in workdir
	dotconfig := fmt.Sprintf("%s/.config", copts.Workdir)

	// Check if the file exists
	// TODO: offer option to start in interactive mode
	if _, err := os.Stat(dotconfig); os.IsNotExist(err) {
		return fmt.Errorf("dotconfig file does not exist: %s", dotconfig)
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := NewProjectOptions(
		nil,
		WithLogger(plog),
		WithWorkingDirectory(copts.Workdir),
		WithDefaultConfigPath(),
		WithPackageManager(&pm),
		WithResolvedPaths(true),
		WithDotConfig(true),
		WithConfig(confOpts),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Set(
		kmake.WithExecOptions(
			exec.WithStdin(copts.IO.In),
		),
	)
}

func (copts *CommandOptions) Unset(confOpts []string) error {
	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Check if dotconfig exists in workdir
	dotconfig := fmt.Sprintf("%s/.config", copts.Workdir)

	// Check if the file exists
	// TODO: offer option to start in interactive mode
	if _, err := os.Stat(dotconfig); os.IsNotExist(err) {
		return fmt.Errorf("dotconfig file does not exist: %s", dotconfig)
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := NewProjectOptions(
		nil,
		WithLogger(plog),
		WithWorkingDirectory(copts.Workdir),
		WithDefaultConfigPath(),
		WithPackageManager(&pm),
		WithResolvedPaths(true),
		WithDotConfig(true),
		WithConfig(confOpts),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	return project.Unset(
		kmake.WithExecOptions(
			exec.WithStdin(copts.IO.In),
		),
	)
}

func (copts *CommandOptions) Build(args CommandBuildArgs) error {
	var err error

	cfgm, err := copts.ConfigManager()
	if err != nil {
		return err
	}

	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Initialize at least the configuration options for a project
	projectOpts, err := NewProjectOptions(
		nil,
		WithLogger(plog),
		WithWorkingDirectory(copts.Workdir),
		WithDefaultConfigPath(),
		WithPackageManager(&pm),
		WithResolvedPaths(true),
		WithDotConfig(false),
	)
	if err != nil {
		return err
	}

	if !IsWorkdirInitialized(copts.Workdir) {
		return fmt.Errorf("cannot build uninitialized project! start with: ukbuild init")
	}

	// Interpret the application
	project, err := NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	parallel := !cfgm.Config.NoParallel
	norender := logger.LoggerTypeFromString(cfgm.Config.Log.Type) != logger.FANCY
	if norender {
		parallel = false
	}

	var missingPacks []pack.Package
	var processes []*paraprogress.Process
	var searches []*processtree.ProcessTreeItem

	_, err = project.Components()
	if err != nil {
		var packages []pack.Package
		search := processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s/%s:%s...", project.Template().Type(), project.Template().Name(), project.Template().Version()), "",
			func(l log.Logger) error {
				// Apply the incoming logger which is tailored to display as a
				// sub-terminal within the fancy processtree.
				pm.ApplyOptions(
					packmanager.WithLogger(l),
				)

				packages, err = pm.Catalog(packmanager.CatalogQuery{
					Name:    project.Template().Name(),
					Types:   []unikraft.ComponentType{unikraft.ComponentTypeApp},
					Version: project.Template().Version(),
					NoCache: args.NoCache,
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
			[]processtree.ProcessTreeOption{
				processtree.WithFailFast(true),
				processtree.WithRenderer(false),
				processtree.WithLogger(plog),
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
			func(l log.Logger, w func(progress float64)) error {
				// Apply the incoming logger which is tailored to display as a
				// sub-terminal within the fancy processtree.
				packages[0].ApplyOptions(
					pack.WithLogger(l),
				)

				return packages[0].Pull(
					pack.WithPullProgressFunc(w),
					pack.WithPullWorkdir(copts.Workdir),
					pack.WithPullLogger(l),
					// pack.WithPullChecksum(!copts.NoChecksum),
					// pack.WithPullCache(!copts.NoCache),
				)
			},
		)

		processes = append(processes, proc)

		paramodel, err := paraprogress.NewParaProgress(
			processes,
			paraprogress.IsParallel(parallel),
			paraprogress.WithRenderer(norender),
			paraprogress.WithLogger(plog),
			paraprogress.WithFailFast(true),
		)
		if err != nil {
			return err
		}

		if err := paramodel.Start(); err != nil {
			return fmt.Errorf("could not pull all components: %v", err)
		}
	}

	templateWorkdir, err := unikraft.PlaceComponent(copts.Workdir, project.Template().Type(), project.Template().Name())
	if err != nil {
		return err
	}

	templateOps, err := NewProjectOptions(
		nil,
		WithLogger(plog),
		WithWorkingDirectory(templateWorkdir),
		WithDefaultConfigPath(),
		WithPackageManager(&pm),
		WithResolvedPaths(true),
		WithDotConfig(false),
	)
	if err != nil {
		return err
	}

	templateProject, err := NewApplicationFromOptions(templateOps)
	if err != nil {
		return err
	}

	project = templateProject.MergeTemplate(project)

	// Overwrite template with user options
	components, err := project.Components()
	if err != nil {
		return err
	}
	for _, component := range components {
		component := component // loop closure

		searches = append(searches, processtree.NewProcessTreeItem(
			fmt.Sprintf("finding %s/%s:%s...", component.Type(), component.Component().Name, component.Component().Version), "",
			func(l log.Logger) error {
				// Apply the incoming logger which is tailored to display as a
				// sub-terminal within the fancy processtree.
				pm.ApplyOptions(
					packmanager.WithLogger(l),
				)

				p, err := pm.Catalog(packmanager.CatalogQuery{
					Name: component.Name(),
					Types: []unikraft.ComponentType{
						unikraft.ComponentTypeCore,
						unikraft.ComponentTypeLib,
						unikraft.ComponentTypePlat,
						unikraft.ComponentTypeArch,
					},
					Version: component.Version(),
					NoCache: args.NoCache,
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
			[]processtree.ProcessTreeOption{
				// processtree.WithVerb("Updating"),
				processtree.IsParallel(parallel),
				// processtree.WithRenderer(norender),
				processtree.WithFailFast(true),
				processtree.WithRenderer(false),
				processtree.WithLogger(plog),
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
				func(l log.Logger, w func(progress float64)) error {
					// Apply the incoming logger which is tailored to display as a
					// sub-terminal within the fancy processtree.
					p.ApplyOptions(
						pack.WithLogger(l),
					)

					return p.Pull(
						pack.WithPullProgressFunc(w),
						pack.WithPullWorkdir(copts.Workdir),
						pack.WithPullLogger(l),
						// pack.WithPullChecksum(!copts.NoChecksum),
						// pack.WithPullCache(!copts.NoCache),
					)
				},
			))
		}

		paramodel, err := paraprogress.NewParaProgress(
			processes,
			paraprogress.IsParallel(parallel),
			paraprogress.WithRenderer(norender),
			paraprogress.WithLogger(plog),
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

	targets, err := project.Targets()
	if err != nil {
		return err
	}

	// Filter the targets by CLI selection
	for _, targ := range targets {
		switch true {
		case
			// If no arguments are supplied
			len(args.Target) == 0 &&
				len(args.Architecture) == 0 &&
				len(args.Platform) == 0,

			// If the --target flag is supplied and the target name match
			len(args.Target) > 0 &&
				targ.Name() == args.Target,

			// If only the --arch flag is supplied and the target's arch matches
			len(args.Architecture) > 0 &&
				len(args.Platform) == 0 &&
				targ.Architecture.Name() == args.Architecture,

			// If only the --plat flag is supplied and the target's platform matches
			len(args.Platform) > 0 &&
				len(args.Architecture) == 0 &&
				targ.Platform.Name() == args.Platform,

			// If both the --arch and --plat flag are supplied and match the target
			len(args.Platform) > 0 &&
				len(args.Architecture) > 0 &&
				targ.Architecture.Name() == args.Architecture &&
				targ.Platform.Name() == args.Platform:

			targets = append(targets, targ)

		default:
			continue
		}
	}

	if len(targets) == 0 {
		plog.Info("no targets to build")
		return nil
	}

	var mopts []kmake.MakeOption
	if args.Jobs > 0 {
		mopts = append(mopts, kmake.WithJobs(args.Jobs))
	} else {
		mopts = append(mopts, kmake.WithMaxJobs(args.Fast))
	}

	for _, targ := range targets {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		targ := targ
		if !project.IsConfigured() && !args.NoConfigure {
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("configuring %s (%s)", targ.Name(), targ.ArchPlatString()),
				func(l log.Logger, w func(progress float64)) error {
					// Apply the incoming logger which is tailored to display as a
					// sub-terminal within the fancy processtree.
					targ.ApplyOptions(
						component.WithLogger(l),
					)

					return project.DefConfig(
						&targ, // Target-specific options
						nil,   // No extra configuration options
						kmake.WithLogger(l),
						// kmake.WithProgressFunc(w),
						kmake.WithSilent(true),
						kmake.WithExecOptions(
							exec.WithStdin(copts.IO.In),
							exec.WithStdout(l.Output()),
							exec.WithStderr(l.Output()),
						),
					)
				},
			))
		}

		if !args.NoPrepare {
			processes = append(processes, paraprogress.NewProcess(
				fmt.Sprintf("preparing %s (%s)", targ.Name(), targ.ArchPlatString()),
				func(l log.Logger, w func(progress float64)) error {
					// Apply the incoming logger which is tailored to display as a
					// sub-terminal within the fancy processtree.
					targ.ApplyOptions(
						component.WithLogger(l),
					)

					return project.Prepare(append(mopts,
						kmake.WithLogger(l),
					)...)
				},
			))
		}

		processes = append(processes, paraprogress.NewProcess(
			fmt.Sprintf("building %s (%s)", targ.Name(), targ.ArchPlatString()),
			func(l log.Logger, w func(progress float64)) error {
				// Apply the incoming logger which is tailored to display as a
				// sub-terminal within the fancy processtree.
				targ.ApplyOptions(
					component.WithLogger(l),
				)

				return project.Build(
					WithBuildLogger(l),
					WithBuildTarget(targ),
					WithBuildProgressFunc(w),
					WithBuildMakeOptions(append(mopts,
						kmake.WithExecOptions(
							exec.WithStdout(l.Output()),
							exec.WithStderr(l.Output()),
						),
					)...),
					WithBuildNoSyncConfig(args.NoSyncConfig),
					WithBuildLogFile(args.SaveBuildLog),
				)
			},
		))
	}

	paramodel, err := paraprogress.NewParaProgress(
		processes,
		// Disable parallelization as:
		//  - The first process may be pulling the container image, which is
		//    necessary for the subsequent build steps;
		//  - The Unikraft build system can re-use compiled files from previous
		//    compilations (if the architecture does not change).
		paraprogress.IsParallel(false),
		paraprogress.WithRenderer(norender),
		paraprogress.WithLogger(plog),
		paraprogress.WithFailFast(true),
	)
	if err != nil {
		return err
	}

	return paramodel.Start()
}

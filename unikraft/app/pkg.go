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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/logger"
	"kraftkit.sh/log"

	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"

	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"

	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/target"

	"kraftkit.sh/utils"
)

type CommandPullArgs struct {
	Manager      string `usage:"Force the handler type (Omition will attempt auto-detect)" default:"auto"`
	Type         string `usage:"Do not use cache when building the image" default:"false"`
	WithDeps     bool   `usage:"Pull dependencies" default:"false"`
	NoDeps       bool   `usage:"Do not pull dependencies" default:"true"`
	Platform     string `usage:"Specify the desired platform"`
	Architecture string `usage:"Specify the desired architecture"`
	AllVersions  bool   `usage:"Pull all versions" default:"false"`
	NoChecksum   bool   `usage:"Do not verify package checksum (if available)" default:"false"`
	NoCache      bool   `usage:"Do not use cache and pull directly from source" default:"false"`
}

type CommandPkgArgs struct {
	Architecture string   `usage:"Filter the creation of the package by architecture of known targets"`
	DotConfig    string   `usage:"Override the path to the KConfig '.config' file"`
	Force        bool     `usage:"Force the use of a packaging handler format" default:"false"`
	Format       string   `usage:"Force the packaging despite possible conflicts" default:"auto"`
	Initrd       string   `usage:"Path to init ramdisk to bundle within the package (passing a path will automatically generate a CPIO image)"`
	Kernel       string   `usage:"Override the path to the unikernel image"`
	KernelDbg    bool     `usage:"Package the debuggable (symbolic) kernel image instead of the stripped image" default:"false"`
	Name         string   `usage:"Specify the name of the package"`
	Output       string   `usage:"Save the package at the following output"`
	Platform     string   `usage:"Filter the creation of the package by platform of known targets"`
	Target       string   `usage:"Package a particular known target"`
	Volumes      []string `usage:"Additional volumes to bundle within the package"`
	WithDbg      bool     `usage:"In addition to the stripped kernel, include the debug image" default:"false"`
}

type CommandListArgs struct {
	LimitResults int  `usage:"Maximum number of items to print (-1 returns all)" default:"30"`
	AsJSON       bool `usage:"Print in JSON format" default:"false"`
	Update       bool `usage:"Get latest information about components before listing results" default:"false"`
	ShowCore     bool `usage:"Show Unikraft core versions" default:"false"`
	ShowArchs    bool `usage:"Show architectures" default:"false"`
	ShowPlats    bool `usage:"Show platforms" default:"false"`
	ShowLibs     bool `usage:"Show libraries" default:"false"`
	ShowApps     bool `usage:"Show applications" default:"false"`
}

func (copts *CommandOptions) initAppPackage(ctx context.Context,
	project *ApplicationConfig,
	targ target.TargetConfig,
	projectOpts *ProjectOptions,
	pm packmanager.PackageManager,
	args *CommandPkgArgs,
) ([]pack.Package, error) {
	var err error

	log, err := copts.Logger()
	if err != nil {
		return nil, err
	}

	log.Tracef("initializing package")

	// Path to the kernel image
	kernel := args.Kernel
	if len(kernel) == 0 {
		kernel = targ.Kernel
	}

	// Prefer the debuggable (symbolic) kernel as the main kernel
	if args.KernelDbg && !args.WithDbg {
		kernel = targ.KernelDbg
	}

	workdir, err := projectOpts.GetWorkingDir()
	if err != nil {
		return nil, err
	}

	name := args.Name

	targets, err := project.Targets()
	if err != nil {
		return nil, err
	}

	// This is a built in naming convention format, which for now allows us to
	// differentiate between different targets.  This should be further discussed
	// the community if this is the best approach.  This can ultimately be
	// overwritten using the --tag flag.
	if len(name) == 0 && len(targets) == 1 {
		name = project.Name()
	} else if len(name) == 0 {
		name = project.Name() + "-" + targ.Name()
	}

	version := project.Version()
	if len(version) == 0 {
		version = "latest"
	}

	extraPackOpts := []pack.PackageOption{
		pack.WithName(name),
		pack.WithVersion(version),
		pack.WithType(unikraft.ComponentTypeApp),
		pack.WithArchitecture(targ.Architecture.Name()),
		pack.WithPlatform(targ.Platform.Name()),
		pack.WithKernel(kernel),
		pack.WithWorkdir(workdir),
		pack.WithLocalLocation(args.Output, args.Force),
	}

	// Options for the initramfs if set
	initrdConfig := targ.Initrd
	if len(args.Initrd) > 0 {
		initrdConfig, err = initrd.ParseInitrdConfig(projectOpts.WorkingDir, args.Initrd)
		if err != nil {
			return nil, fmt.Errorf("could not parse --initrd flag with value %s: %s", args.Initrd, err)
		}
	}

	// Warn if potentially missing configuration options
	// if initrdConfig != nil && unikraft.EnabledInitramfs()
	extraPackOpts = append(extraPackOpts,
		pack.WithInitrdConfig(initrdConfig),
	)

	packOpts, err := pack.NewPackageOptions(extraPackOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not prepare package for target: %s: %v", targ.Name(), err)
	}

	// Switch the package manager the desired format for this target
	if len(targ.Format) > 0 && targ.Format != "auto" {
		if pm.Format() == "umbrella" {
			pm, err = pm.From(targ.Format)
			if err != nil {
				return nil, err
			}

			// Skip this target as we cannot package it
		} else if pm.Format() != targ.Format && !args.Force {
			log.Warn("skipping %s target %s", targ.Format, targ.Name)
			return nil, nil
		}
	}

	pack, err := pm.NewPackageFromOptions(ctx, packOpts)
	if err != nil {
		return nil, fmt.Errorf("could not initialize package: %s", err)
	}

	return pack, nil
}

func (copts *CommandOptions) Pull(args CommandPullArgs, query string) error {
	var err error
	var project *ApplicationConfig
	var processes []*paraprogress.Process
	var queries []packmanager.CatalogQuery

	workdir := copts.Workdir

	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Force a particular package manager
	if len(args.Manager) > 0 && args.Manager != "auto" {
		pm, err = pm.From(args.Manager)
		if err != nil {
			return err
		}
	}

	// Are we pulling an application directory?  If so, interpret the application
	// so we can get a list of components
	if f, err := os.Stat(query); err == nil && f.IsDir() {
		workdir = query
		projectOpts, err := NewProjectOptions(
			nil,
			WithLogger(plog),
			WithWorkingDirectory(workdir),
			WithDefaultConfigPath(),
			WithResolvedPaths(true),
		)
		if err != nil {
			return err
		}

		// Interpret the application
		project, err := NewApplicationFromOptions(projectOpts)
		if err != nil {
			return err
		}

		_, err = project.Components()
		if err != nil {
			// Pull the template from the package manager
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
						pack.WithPullWorkdir(workdir),
						pack.WithPullLogger(l),
						// pack.WithPullChecksum(!opts.NoChecksum),
						// pack.WithPullCache(!opts.NoCache),
					)
				},
			)

			processes = append(processes, proc)

			paramodel, err := paraprogress.NewParaProgress(
				processes,
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

		templateWorkdir, err := unikraft.PlaceComponent(workdir, project.Template().Type(), project.Template().Name())
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
		next, err := pm.Catalog(c)
		if err != nil {
			return err
		}

		if len(next) == 0 {
			plog.Warnf("could not find %s", c.String())
			continue
		}

		for _, p := range next {
			p := p
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
						pack.WithPullWorkdir(workdir),
						pack.WithPullLogger(l),
						pack.WithPullChecksum(!args.NoChecksum),
						pack.WithPullCache(!args.NoCache),
					)
				},
			))
		}
	}

	model, err := paraprogress.NewParaProgress(
		processes,
		paraprogress.IsParallel(true),
		paraprogress.WithLogger(plog),
	)
	if err != nil {
		return err
	}

	if err := model.Start(); err != nil {
		return err
	}

	if project != nil {
		project.PrintInfo(copts.IO)
	}

	return nil
}

func (copts *CommandOptions) Source(source string) error {
	var err error

	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	pm, err = pm.IsCompatible(source)
	if err != nil {
		return err
	}

	if err = pm.AddSource(source); err != nil {
		return err
	}

	return nil
}

func (copts *CommandOptions) Pkg(args CommandPkgArgs) error {
	var err error

	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	// Force a particular package manager
	if len(args.Format) > 0 && args.Format != "auto" {
		pm, err = pm.From(args.Format)
		if err != nil {
			return err
		}
	}

	projectOpts, err := NewProjectOptions(
		nil,
		WithLogger(plog),
		WithWorkingDirectory(copts.Workdir),
		WithDefaultConfigPath(),
		WithPackageManager(&pm),
		WithResolvedPaths(true),
		WithDotConfig(true),
	)
	if err != nil {
		return err
	}

	// Interpret the application
	project, err := NewApplicationFromOptions(projectOpts)
	if err != nil {
		return err
	}

	ctx := context.Background()
	var packages []pack.Package

	// Generate a package for every matching requested target
	targets, err := project.Targets()
	if err != nil {
		return err
	}
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

			packs, err := copts.initAppPackage(ctx, project, targ, projectOpts, pm, &args)
			if err != nil {
				return fmt.Errorf("could not create package: %s", err)
			}

			packages = append(packages, packs...)

		default:
			continue
		}
	}

	if len(packages) == 0 {
		plog.Info("nothing to package")
		return nil
	}

	cfgm, err := copts.ConfigManager()
	if err != nil {
		return err
	}

	parallel := !cfgm.Config.NoParallel
	norender := logger.LoggerTypeFromString(cfgm.Config.Log.Type) != logger.FANCY
	if norender {
		parallel = false
	} else {
		plog.SetOutput(ioutil.Discard)
	}

	var tree []*processtree.ProcessTreeItem
	for _, p := range packages {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		p := p

		tree = append(tree, processtree.NewProcessTreeItem(
			"Packaging "+p.CanonicalName(),
			p.Options().ArchPlatString(),
			func(l log.Logger) error {
				// Apply the incoming logger which is tailored to display as a
				// sub-terminal within the fancy processtree.
				p.ApplyOptions(
					pack.WithLogger(l),
				)

				return p.Pack()
			},
		))
	}

	model, err := processtree.NewProcessTree(
		[]processtree.ProcessTreeOption{
			processtree.WithVerb("Packaging..."),
			processtree.IsParallel(parallel),
			processtree.WithRenderer(norender),
			processtree.WithLogger(plog),
		},
		tree...,
	)
	if err != nil {
		return err
	}

	return model.Start()
}

func (copts *CommandOptions) List(args CommandListArgs) error {
	var err error

	pm, err := copts.PackageManager()
	if err != nil {
		return err
	}

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	query := packmanager.CatalogQuery{}
	if args.ShowCore {
		query.Types = append(query.Types, unikraft.ComponentTypeCore)
	}
	if args.ShowArchs {
		query.Types = append(query.Types, unikraft.ComponentTypeArch)
	}
	if args.ShowPlats {
		query.Types = append(query.Types, unikraft.ComponentTypePlat)
	}
	if args.ShowLibs {
		query.Types = append(query.Types, unikraft.ComponentTypeLib)
	}
	if args.ShowApps {
		query.Types = append(query.Types, unikraft.ComponentTypeApp)
	}

	var packages []pack.Package

	// List pacakges part of a project
	if len(copts.Workdir) > 0 {
		projectOpts, err := NewProjectOptions(
			nil,
			WithLogger(plog),
			WithWorkingDirectory(copts.Workdir),
			WithDefaultConfigPath(),
			WithPackageManager(&pm),
		)
		if err != nil {
			return err
		}

		// Interpret the application
		app, err := NewApplicationFromOptions(projectOpts)
		if err != nil {
			return err
		}

		app.PrintInfo(copts.IO)

	} else {
		packages, err = pm.Catalog(query,
			pack.WithWorkdir(copts.Workdir),
		)
		if err != nil {
			return err
		}
	}

	err = copts.IO.StartPager()
	if err != nil {
		plog.Errorf("error starting pager: %v", err)
	}

	defer copts.IO.StopPager()

	cs := copts.IO.ColorScheme()
	table := utils.NewTablePrinter(copts.IO)

	// Header row
	table.AddField("TYPE", nil, cs.Bold)
	table.AddField("PACKAGE", nil, cs.Bold)
	table.AddField("LATEST", nil, cs.Bold)
	table.AddField("FORMAT", nil, cs.Bold)
	table.EndRow()

	for _, pack := range packages {
		table.AddField(string(pack.Options().Type), nil, nil)
		table.AddField(pack.Name(), nil, nil)
		table.AddField(pack.Options().Version, nil, nil)
		table.AddField(pack.Format(), nil, nil)
		table.EndRow()
	}

	return table.Render()
}

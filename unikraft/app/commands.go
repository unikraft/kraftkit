// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Cezar Craciunoiu <cezar@unikraft.io>
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
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/moby/moby/pkg/namesgenerator"
	"github.com/pkg/errors"
	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/logger"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/log"
	"kraftkit.sh/machine"
	machinedriver "kraftkit.sh/machine/driver"
	machinedriveropts "kraftkit.sh/machine/driveropts"
	"kraftkit.sh/machine/qemu/qmp"
	kmake "kraftkit.sh/make"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/tui/paraprogress"
	"kraftkit.sh/tui/processtree"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/target"
	"kraftkit.sh/utils"
)

type CommandOptions struct {
	PackageManager func(opts ...packmanager.PackageManagerOption) (packmanager.PackageManager, error)
	ConfigManager  func() (*config.ConfigManager, error)
	Logger         func() (log.Logger, error)
	IO             *iostreams.IOStreams
	Workdir        string
}

type CommandBuildArgs struct {
	NoCache      bool
	Architecture string
	Platform     string
	DotConfig    string
	Target       string
	KernelDbg    bool
	Fast         bool
	Jobs         int
	NoSyncConfig bool
	NoPrepare    bool
	NoFetch      bool
	NoPull       bool
	NoConfigure  bool
	SaveBuildLog string
}

type CommandListArgs struct {
	LimitResults int
	AsJSON       bool
	Update       bool
	ShowCore     bool
	ShowArchs    bool
	ShowPlats    bool
	ShowLibs     bool
	ShowApps     bool
}

type CommandPullArgs struct {
	Manager      string
	Type         string
	WithDeps     bool
	Workdir      string
	NoDeps       bool
	Platform     string
	Architecture string
	AllVersions  bool
	NoChecksum   bool
	NoCache      bool
}

type CommandPkgArgs struct {
	Architecture string
	DotConfig    string
	Force        bool
	Format       string
	Initrd       string
	Kernel       string
	KernelDbg    bool
	Name         string
	Output       string
	Platform     string
	Target       string
	Volumes      []string
	WithDbg      bool
}

type CommandPsArgs struct {
	ShowAll      bool
	Hypervisor   string
	Architecture string
	Platform     string
	Quiet        bool
	Long         bool
}

type CommandRunArgs struct {
	Architecture  string
	Detach        bool
	DisableAccel  bool
	Hypervisor    string
	Memory        int
	NoMonitor     bool
	PinCPUs       string
	Platform      string
	Remove        bool
	Target        string
	Volumes       []string
	WithKernelDbg bool
}

type CommandEventsArgs struct {
	QuitTogether bool
	Granularity  time.Duration
}

type machineWaitGroup struct {
	lock sync.RWMutex
	mids []machine.MachineID
}

func (mwg *machineWaitGroup) Done(needle machine.MachineID) {
	mwg.lock.Lock()
	defer mwg.lock.Unlock()

	if !mwg.Contains(needle) {
		return
	}

	for i, mid := range mwg.mids {
		if mid == needle {
			mwg.mids = append(mwg.mids[:i], mwg.mids[i+1:]...)
			return
		}
	}
}

func (mwg *machineWaitGroup) Wait() {
	for {
		if len(mwg.mids) == 0 {
			break
		}
	}
}

func (mwg *machineWaitGroup) Contains(needle machine.MachineID) bool {
	for _, mid := range mwg.mids {
		if mid == needle {
			return true
		}
	}

	return false
}

func (mwg *machineWaitGroup) Add(mid machine.MachineID) {
	mwg.lock.Lock()
	defer mwg.lock.Unlock()

	if mwg.Contains(mid) {
		return
	}

	mwg.mids = append(mwg.mids, mid)
}

var (
	observations = new(machineWaitGroup)
	drivers      = make(map[machinedriver.DriverType]machinedriver.Driver)
)

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

func (copts *CommandOptions) Ps(args CommandPsArgs, ExtraArgs ...string) error {
	var err error

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	cfgm, err := copts.ConfigManager()
	if err != nil {
		return err
	}

	var onlyDriverType *machinedriver.DriverType
	if args.Hypervisor == "all" {
		dt, err := machinedriver.DetectHostHypervisor()
		if err != nil {
			return err
		}
		onlyDriverType = &dt
	} else {
		if !utils.Contains(machinedriver.DriverNames(), args.Hypervisor) {
			return fmt.Errorf("unknown hypervisor driver: %s", args.Hypervisor)
		}
	}

	type psTable struct {
		id      machine.MachineID
		image   string
		args    string
		created string
		status  machine.MachineState
		mem     string
		arch    string
		plat    string
		driver  string
	}

	var items []psTable

	store, err := machine.NewMachineStoreFromPath(cfgm.Config.RuntimeDir)
	if err != nil {
		return err
	}

	mids, err := store.ListAllMachineConfigs()
	if err != nil {
		return err
	}

	ctx := context.Background()

	drivers := make(map[machinedriver.DriverType]machinedriver.Driver)

	for mid, mopts := range mids {
		if onlyDriverType != nil && mopts.DriverName != onlyDriverType.String() {
			continue
		}

		driverType := machinedriver.DriverTypeFromName(mopts.DriverName)
		if driverType == machinedriver.UnknownDriver {
			plog.Warnf("unknown driver %s for %s", driverType.String(), mid)
			continue
		}

		if _, ok := drivers[driverType]; !ok {
			driver, err := machinedriver.New(driverType,
				machinedriveropts.WithRuntimeDir(cfgm.Config.RuntimeDir),
				machinedriveropts.WithMachineStore(store),
			)
			if err != nil {
				return err
			}

			drivers[driverType] = driver
		}

		driver := drivers[driverType]

		state, err := driver.State(ctx, mid)
		if err != nil {
			return err
		}

		if !args.ShowAll && state != machine.MachineStateRunning {
			continue
		}

		items = append(items, psTable{
			id:      mid,
			args:    strings.Join(mopts.Arguments, " "),
			image:   mopts.Source,
			status:  state,
			mem:     strconv.FormatUint(mopts.MemorySize, 10) + "MB",
			created: humanize.Time(mopts.CreatedAt),
			arch:    mopts.Architecture,
			plat:    mopts.Platform,
			driver:  mopts.DriverName,
		})
	}

	err = copts.IO.StartPager()
	if err != nil {
		plog.Errorf("error starting pager: %v", err)
	}

	defer copts.IO.StopPager()

	cs := copts.IO.ColorScheme()
	table := utils.NewTablePrinter(copts.IO)

	// Header row
	table.AddField("MACHINE ID", nil, cs.Bold)
	table.AddField("IMAGE", nil, cs.Bold)
	table.AddField("ARGS", nil, cs.Bold)
	table.AddField("CREATED", nil, cs.Bold)
	table.AddField("STATUS", nil, cs.Bold)
	table.AddField("MEM", nil, cs.Bold)
	if args.Long {
		table.AddField("ARCH", nil, cs.Bold)
		table.AddField("PLAT", nil, cs.Bold)
		table.AddField("DRIVER", nil, cs.Bold)
	}
	table.EndRow()

	for _, item := range items {
		table.AddField(item.id.ShortString(), nil, nil)
		table.AddField(item.image, nil, nil)
		table.AddField(item.args, nil, nil)
		table.AddField(item.created, nil, nil)
		table.AddField(item.status.String(), nil, nil)
		table.AddField(item.mem, nil, nil)
		if args.Long {
			table.AddField(item.arch, nil, nil)
			table.AddField(item.plat, nil, nil)
			table.AddField(item.driver, nil, nil)
		}
		table.EndRow()
	}

	return table.Render()
}

func (copts *CommandOptions) Remove(args ...string) error {
	var err error

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	cfgm, err := copts.ConfigManager()
	if err != nil {
		return err
	}

	ctx := context.Background()
	store, err := machine.NewMachineStoreFromPath(cfgm.Config.RuntimeDir)
	if err != nil {
		return fmt.Errorf("could not access machine store: %v", err)
	}

	allMids, err := store.ListAllMachineIDs()
	if err != nil {
		return fmt.Errorf("could not list machines: %v", err)
	}

	var mids []machine.MachineID

	for _, mid1 := range args {
		found := false
		for _, mid2 := range allMids {
			if mid1 == mid2.ShortString() || mid1 == mid2.String() {
				mids = append(mids, mid2)
				found = true
			}
		}

		if !found {
			return fmt.Errorf("could not find machine %s", mid1)
		}
	}

	for _, mid := range mids {
		mid := mid // loop closure

		if observations.Contains(mid) {
			continue
		}

		observations.Add(mid)

		go func() {
			observations.Add(mid)

			plog.Infof("removing %s...", mid.ShortString())

			mcfg := &machine.MachineConfig{}
			if err := store.LookupMachineConfig(mid, mcfg); err != nil {
				plog.Errorf("could not look up machine config: %v", err)
				observations.Done(mid)
				return
			}

			driverType := machinedriver.DriverTypeFromName(mcfg.DriverName)

			if _, ok := drivers[driverType]; !ok {
				driver, err := machinedriver.New(driverType,
					machinedriveropts.WithLogger(plog),
					machinedriveropts.WithMachineStore(store),
					machinedriveropts.WithRuntimeDir(cfgm.Config.RuntimeDir),
				)
				if err != nil {
					plog.Errorf("could not instantiate machine driver for %s: %v", mid.ShortString(), err)
					observations.Done(mid)
					return
				}

				drivers[driverType] = driver
			}

			driver := drivers[driverType]

			if err := driver.Destroy(ctx, mid); err != nil {
				plog.Errorf("could not remove machine %s: %v", mid.ShortString(), err)
			} else {
				plog.Infof("removed %s", mid.ShortString())
			}

			observations.Done(mid)
		}()
	}

	observations.Wait()

	return nil
}

func (copts *CommandOptions) Run(args CommandRunArgs, ExtraArgs ...string) error {
	var err error

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	cfgm, err := copts.ConfigManager()
	if err != nil {
		return err
	}

	var driverType *machinedriver.DriverType
	if args.Hypervisor == "auto" {
		dt, err := machinedriver.DetectHostHypervisor()
		if err != nil {
			return err
		}
		driverType = &dt
	} else if args.Hypervisor == "config" {
		args.Hypervisor = cfgm.Config.DefaultPlat
	}

	if driverType == nil && len(args.Hypervisor) > 0 && !utils.Contains(machinedriver.DriverNames(), args.Hypervisor) {
		return fmt.Errorf("unknown hypervisor driver: %s", args.Hypervisor)
	}

	debug := logger.LogLevelFromString(cfgm.Config.Log.Level) >= logger.DEBUG
	var msopts []machine.MachineStoreOption
	if debug {
		msopts = append(msopts,
			machine.WithMachineStoreLogger(plog),
		)
	}

	store, err := machine.NewMachineStoreFromPath(cfgm.Config.RuntimeDir, msopts...)
	if err != nil {
		return fmt.Errorf("could not access machine store: %v", err)
	}

	driver, err := machinedriver.New(*driverType,
		machinedriveropts.WithBackground(args.Detach),
		machinedriveropts.WithRuntimeDir(cfgm.Config.RuntimeDir),
		machinedriveropts.WithMachineStore(store),
		machinedriveropts.WithLogger(plog),
		machinedriveropts.WithDebug(debug),
		machinedriveropts.WithExecOptions(
			exec.WithStdout(os.Stdout),
			exec.WithStderr(os.Stderr),
		),
	)
	if err != nil {
		return err
	}

	mopts := []machine.MachineOption{
		machine.WithDriverName(driverType.String()),
		machine.WithDestroyOnExit(args.Remove),
	}

	// The following sequence checks the position argument of `kraft run ENTITY`
	// where ENTITY can either be:
	// a). path to a project which either uses the only specified target or one
	//     specified via the -t flag;
	// b). a target defined within the context of `workdir` (which is either set
	//     via -w or is the current working directory); or
	// c). path to a kernel.
	var workdir string
	var entity string
	var kernelArgs []string

	// Determine if more than one positional arguments have been provided.  If
	// this is the case, everything after the first position argument are kernel
	// parameters which should be passed appropriately.
	if len(ExtraArgs) > 1 {
		entity = ExtraArgs[0]
		kernelArgs = ExtraArgs[1:]
	} else if len(ExtraArgs) == 1 {
		entity = ExtraArgs[0]
	}

	// a). path to a project
	if len(entity) > 0 && IsWorkdirInitialized(entity) {
		workdir = entity

		// Otherwise use the current working directory
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		if IsWorkdirInitialized(cwd) {
			workdir = cwd
			kernelArgs = ExtraArgs
		}
	}

	// b). use a defined working directory as a Unikraft project
	if len(workdir) > 0 {
		target := args.Target
		projectOpts, err := NewProjectOptions(
			nil,
			WithLogger(plog),
			WithWorkingDirectory(workdir),
			WithDefaultConfigPath(),
			// WithPackageManager(&pm),
			WithResolvedPaths(true),
			WithDotConfig(false),
		)
		if err != nil {
			return err
		}

		// Interpret the application
		app, err := NewApplicationFromOptions(projectOpts)
		if err != nil {
			return err
		}

		if len(app.TargetNames()) == 1 {
			target = app.TargetNames()[0]
			if len(args.Target) > 0 && args.Target != target {
				return fmt.Errorf("unknown target: %s", args.Target)
			}

		} else if len(target) == 0 && len(app.TargetNames()) > 1 {
			if cfgm.Config.NoPrompt {
				return fmt.Errorf("with 'no prompt' enabled please select a target")
			}

			sp := selection.New("select target:",
				selection.Choices(app.TargetNames()),
			)
			sp.Filter = nil

			selectedTarget, err := sp.RunPrompt()
			if err != nil {
				return err
			}

			target = selectedTarget.String

		} else if target != "" && utils.Contains(app.TargetNames(), target) {
			return fmt.Errorf("unknown target: %s", target)
		}

		t, err := app.TargetByName(target)
		if err != nil {
			return err
		}

		// Validate selection of target
		if len(args.Architecture) > 0 && (args.Architecture != t.Architecture.Name()) {
			return fmt.Errorf("selected target (%s) does not match specified architecture (%s)", t.ArchPlatString(), args.Architecture)
		}
		if len(args.Platform) > 0 && (args.Platform != t.Platform.Name()) {
			return fmt.Errorf("selected target (%s) does not match specified platform (%s)", t.ArchPlatString(), args.Platform)
		}

		mopts = append(mopts,
			machine.WithArchitecture(t.Architecture.Name()),
			machine.WithPlatform(t.Platform.Name()),
			machine.WithName(machine.MachineName(t.Name())),
			machine.WithAcceleration(!args.DisableAccel),
			machine.WithSource("project://"+app.Name()+":"+t.Name()),
		)

		// Use the symbolic debuggable kernel image?
		if args.WithKernelDbg {
			mopts = append(mopts, machine.WithKernel(t.KernelDbg))
		} else {
			mopts = append(mopts, machine.WithKernel(t.Kernel))
		}

		// If no entity was set earlier and we're not within the context of a working
		// directory, then we're unsure what to run
	} else if len(entity) == 0 {
		return fmt.Errorf("cannot run without providing a working directory (and target), kernel binary or package")

		// c). Is the provided first position argument a binary image?
	} else if f, err := os.Stat(entity); err == nil && !f.IsDir() {
		if len(args.Architecture) == 0 || len(args.Platform) == 0 {
			return fmt.Errorf("cannot use `kraft run KERNEL` without specifying --arch and --plat")
		}

		mopts = append(mopts,
			machine.WithArchitecture(args.Architecture),
			machine.WithPlatform(args.Platform),
			machine.WithName(machine.MachineName(namesgenerator.GetRandomName(0))),
			machine.WithKernel(entity),
			machine.WithSource("kernel://"+filepath.Base(entity)),
		)
	} else {
		return fmt.Errorf("could not determine what to run: %s", entity)
	}

	mopts = append(mopts,
		machine.WithMemorySize(uint64(args.Memory)),
		machine.WithArguments(kernelArgs),
	)

	ctx := context.Background()

	// Create the machine
	mid, err := driver.Create(ctx, mopts...)
	if err != nil {
		return err
	}

	plog.Infof("created %s instance %s", driverType.String(), mid.ShortString())

	// Start the machine
	if err := driver.Start(ctx, mid); err != nil {
		return err
	}

	if !args.NoMonitor {
		// Spawn an event monitor or attach to an existing monitor
		_, err = os.Stat(cfgm.Config.EventsPidFile)
		if err != nil && os.IsNotExist(err) {
			plog.Debugf("launching event monitor...")

			// Spawn and detach a new events monitor
			e, err := exec.NewExecutable(os.Args[0], nil, "events", "--quit-together")
			if err != nil {
				return err
			}

			process, err := exec.NewProcessFromExecutable(e,
				exec.WithDetach(true),
			)
			if err != nil {
				return err
			}

			if err := process.Start(); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		// TODO: Failsafe check if a a pidfile for the events monitor exists, let's
		// check that it is active
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctrlc := make(chan os.Signal, 1)
	signal.Notify(ctrlc, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-ctrlc // wait for Ctrl+C
		fmt.Printf("\n")

		// Remove the instance on Ctrl+C if the --rm flag is passed
		if args.Remove {
			plog.Infof("removing %s...", mid.ShortString())
			if err := driver.Destroy(ctx, mid); err != nil {
				plog.Errorf("could not remove %s: %v", mid, err)
			}
		}

		cancel()
	}()

	// Tail the logs if -d|--detach is not provided
	if !args.Detach {
		plog.Infof("starting to tail %s logs...", mid.ShortString())

		go func() {
			events, errs, err := driver.ListenStatusUpdate(ctx, mid)
			if err != nil {
				plog.Errorf("could not listen for machine updates: %v", err)
				ctrlc <- syscall.SIGTERM
				if !args.Remove {
					cancel()
				}
			}

			for {
				// Wait on either channel
				select {
				case status := <-events:
					switch status {
					case machine.MachineStateExited, machine.MachineStateDead:
						ctrlc <- syscall.SIGTERM
						if !args.Remove {
							cancel()
						}
						return
					}

				case err := <-errs:
					plog.Errorf("received event error: %v", err)
					return
				}
			}
		}()

		driver.TailWriter(ctx, mid, copts.IO.Out)
	}

	return nil
}

func (copts *CommandOptions) Stop(args ...string) error {
	var err error

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	cfgm, err := copts.ConfigManager()
	if err != nil {
		return err
	}

	ctx := context.Background()
	store, err := machine.NewMachineStoreFromPath(cfgm.Config.RuntimeDir)
	if err != nil {
		return fmt.Errorf("could not access machine store: %v", err)
	}

	allMids, err := store.ListAllMachineIDs()
	if err != nil {
		return fmt.Errorf("could not list machines: %v", err)
	}

	var mids []machine.MachineID

	for _, mid1 := range args {
		found := false
		for _, mid2 := range allMids {
			if mid1 == mid2.ShortString() || mid1 == mid2.String() {
				mids = append(mids, mid2)
				found = true
			}
		}

		if !found {
			return fmt.Errorf("could not find machine %s", mid1)
		}
	}

	for _, mid := range mids {
		mid := mid // loop closure

		if observations.Contains(mid) {
			continue
		}

		observations.Add(mid)

		go func() {
			observations.Add(mid)

			plog.Infof("stopping %s...", mid.ShortString())

			state, err := store.LookupMachineState(mid)
			if err != nil {
				plog.Errorf("could not look up machine state: %v", err)
				observations.Done(mid)
				return
			}

			switch state {
			case machine.MachineStateDead, machine.MachineStateExited:
				plog.Errorf("%s has exited", mid.ShortString())
				observations.Done(mid)
				return
			}

			mcfg := &machine.MachineConfig{}
			if err := store.LookupMachineConfig(mid, mcfg); err != nil {
				plog.Errorf("could not look up machine config: %v", err)
				observations.Done(mid)
				return
			}

			driverType := machinedriver.DriverTypeFromName(mcfg.DriverName)

			if _, ok := drivers[driverType]; !ok {
				driver, err := machinedriver.New(driverType,
					machinedriveropts.WithLogger(plog),
					machinedriveropts.WithMachineStore(store),
					machinedriveropts.WithRuntimeDir(cfgm.Config.RuntimeDir),
				)
				if err != nil {
					plog.Errorf("could not instantiate machine driver for %s: %v", mid.ShortString(), err)
					observations.Done(mid)
					return
				}

				drivers[driverType] = driver
			}

			driver := drivers[driverType]

			if err := driver.Stop(ctx, mid); err != nil {
				plog.Errorf("could not stop machine %s: %v", mid.ShortString(), err)
			} else {
				plog.Infof("stopped %s", mid.ShortString())
			}

			observations.Done(mid)
		}()
	}

	observations.Wait()

	return nil
}

func (copts *CommandOptions) Events(args CommandEventsArgs, extraArgs ...string) error {
	var err error

	plog, err := copts.Logger()
	if err != nil {
		return err
	}

	cfgm, err := copts.ConfigManager()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	store, err := machine.NewMachineStoreFromPath(cfgm.Config.RuntimeDir)
	if err != nil {
		cancel()
		return fmt.Errorf("could not access machine store: %v", err)
	}

	var pidfile *os.File

	// Check if a pid has already been enabled
	if _, err := os.Stat(cfgm.Config.EventsPidFile); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(cfgm.Config.EventsPidFile), 0o755); err != nil {
			cancel()
			return err
		}

		pidfile, err = os.OpenFile(cfgm.Config.EventsPidFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666)
		if err != nil {
			cancel()
			return fmt.Errorf("could not create pidfile: %v", err)
		}

		defer func() {
			pidfile.Close()

			if err := os.Remove(cfgm.Config.EventsPidFile); err != nil {
				plog.Errorf("could not remove pid file: %v", err)
			}
		}()

		pidfile.Write([]byte(fmt.Sprintf("%d", os.Getpid())))

		if err := pidfile.Sync(); err != nil {
			cancel()
			return fmt.Errorf("could not sync pid file: %v", err)
		}
	}

	// Handle Ctrl+C of the event monitor
	ctrlc := make(chan os.Signal, 1)
	signal.Notify(ctrlc, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctrlc // wait for Ctrl+C
		cancel()
	}()

	// TODO: Should we thrown an error here if a process file already exists? We
	// use a pid file for `kraft run` to continuously monitor running machines.

	// Actively seek for machines whose events we wish to monitor.  The thread
	// will continuously read from the machine store which can be updated
	// elsewhere and acts as the source-of-truth for VMs which are being
	// instantiated by KraftKit.  The thread dies if there is nothing in the store
	// and the `--quit-together` flag is set.
seek:
	for {
		select {
		case <-ctx.Done():
			break seek
		default:
		}

		var mids []machine.MachineID
		allMids, err := store.ListAllMachineIDs()
		if err != nil {
			return fmt.Errorf("could not list machines: %v", err)
		}

		if len(extraArgs) > 0 {
			for _, mid := range allMids {
				if extraArgs[0] == mid.String() || extraArgs[0] == mid.ShortString() {
					mids = append(mids, mid)
				}
			}
		} else {
			mids = allMids
		}

		if len(mids) == 0 && args.QuitTogether {
			break
		}

		for _, mid := range mids {
			mid := mid // loop closure

			state, err := store.LookupMachineState(mid)
			if err != nil {
				plog.Errorf("could not look up machine state: %v", err)
				continue
			}

			switch state {
			case machine.MachineStateDead,
				machine.MachineStateExited,
				machine.MachineStateUnknown:
				continue
			default:
			}

			if observations.Contains(mid) {
				continue
			}

			plog.Infof("monitoring %s", mid.ShortString())

			var mcfg machine.MachineConfig
			if err := store.LookupMachineConfig(mid, &mcfg); err != nil {
				plog.Errorf("could not look up machine config: %v", err)
				continue
			}

			go func() {
				observations.Add(mid)

				if args.QuitTogether {
					defer observations.Done(mid)
				}

				mcfg := &machine.MachineConfig{}
				if err := store.LookupMachineConfig(mid, mcfg); err != nil {
					plog.Errorf("could not look up machine config: %v", err)
					observations.Done(mid)
					return
				}

				driverType := machinedriver.DriverTypeFromName(mcfg.DriverName)

				if _, ok := drivers[driverType]; !ok {
					driver, err := machinedriver.New(driverType,
						machinedriveropts.WithLogger(plog),
						machinedriveropts.WithMachineStore(store),
						machinedriveropts.WithRuntimeDir(cfgm.Config.RuntimeDir),
					)
					if err != nil {
						plog.Errorf("could not instantiate machine driver for %s: %v", mid, err)
						observations.Done(mid)
						return
					}

					drivers[driverType] = driver
				}

				driver := drivers[driverType]

				events, errs, err := driver.ListenStatusUpdate(ctx, mid)
				if err != nil {
					plog.Warnf("could not listen for status updates for %s: %v", mid.ShortString(), err)

					// Check the state of the machine using the driver, for a more
					// accurate read
					state, err := driver.State(ctx, mid)
					if err != nil {
						plog.Errorf("could not look up machine state: %v", err)
					}

					switch state {
					case machine.MachineStateExited, machine.MachineStateDead:
						if mcfg.DestroyOnExit {
							plog.Infof("removing %s...", mid.ShortString())
							if err := driver.Destroy(ctx, mid); err != nil {
								plog.Errorf("could not remove machine: %v: ", err)
							}
						}
					}

					observations.Done(mid)
					return
				}

				for {
					// Wait on either channel
					select {
					case state := <-events:
						plog.Infof("%s : %s", mid.ShortString(), state.String())
						switch state {
						case machine.MachineStateExited, machine.MachineStateDead:
							if mcfg.DestroyOnExit {
								plog.Infof("removing %s...", mid.ShortString())
								if err := driver.Destroy(ctx, mid); err != nil {
									plog.Errorf("could not remove machine: %v: ", err)
								}
							}
							observations.Done(mid)
							return
						}

					case err := <-errs:
						if !errors.Is(err, qmp.ErrAcceptedNonEvent) {
							plog.Errorf("%v", err)
						}
						observations.Done(mid)

					case <-ctx.Done():
						observations.Done(mid)
						return
					}
				}
			}()
		}

		time.Sleep(time.Second * args.Granularity)
	}

	observations.Wait()

	return nil
}

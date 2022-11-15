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
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/erikgeiser/promptkit/selection"
	"github.com/moby/moby/pkg/namesgenerator"
	"kraftkit.sh/exec"
	"kraftkit.sh/internal/logger"
	"kraftkit.sh/machine"
	machinedriver "kraftkit.sh/machine/driver"
	machinedriveropts "kraftkit.sh/machine/driveropts"
	"kraftkit.sh/machine/qemu/qmp"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/utils"
)

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

		if machinedriver.Observations.Contains(mid) {
			continue
		}

		machinedriver.Observations.Add(mid)

		go func() {
			machinedriver.Observations.Add(mid)

			plog.Infof("removing %s...", mid.ShortString())

			mcfg := &machine.MachineConfig{}
			if err := store.LookupMachineConfig(mid, mcfg); err != nil {
				plog.Errorf("could not look up machine config: %v", err)
				machinedriver.Observations.Done(mid)
				return
			}

			driverType := machinedriver.DriverTypeFromName(mcfg.DriverName)

			if _, ok := machinedriver.Drivers[driverType]; !ok {
				driver, err := machinedriver.New(driverType,
					machinedriveropts.WithLogger(plog),
					machinedriveropts.WithMachineStore(store),
					machinedriveropts.WithRuntimeDir(cfgm.Config.RuntimeDir),
				)
				if err != nil {
					plog.Errorf("could not instantiate machine driver for %s: %v", mid.ShortString(), err)
					machinedriver.Observations.Done(mid)
					return
				}

				machinedriver.Drivers[driverType] = driver
			}

			driver := machinedriver.Drivers[driverType]

			if err := driver.Destroy(ctx, mid); err != nil {
				plog.Errorf("could not remove machine %s: %v", mid.ShortString(), err)
			} else {
				plog.Infof("removed %s", mid.ShortString())
			}

			machinedriver.Observations.Done(mid)
		}()
	}

	machinedriver.Observations.Wait()

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

		if machinedriver.Observations.Contains(mid) {
			continue
		}

		machinedriver.Observations.Add(mid)

		go func() {
			machinedriver.Observations.Add(mid)

			plog.Infof("stopping %s...", mid.ShortString())

			state, err := store.LookupMachineState(mid)
			if err != nil {
				plog.Errorf("could not look up machine state: %v", err)
				machinedriver.Observations.Done(mid)
				return
			}

			switch state {
			case machine.MachineStateDead, machine.MachineStateExited:
				plog.Errorf("%s has exited", mid.ShortString())
				machinedriver.Observations.Done(mid)
				return
			}

			mcfg := &machine.MachineConfig{}
			if err := store.LookupMachineConfig(mid, mcfg); err != nil {
				plog.Errorf("could not look up machine config: %v", err)
				machinedriver.Observations.Done(mid)
				return
			}

			driverType := machinedriver.DriverTypeFromName(mcfg.DriverName)

			if _, ok := machinedriver.Drivers[driverType]; !ok {
				driver, err := machinedriver.New(driverType,
					machinedriveropts.WithLogger(plog),
					machinedriveropts.WithMachineStore(store),
					machinedriveropts.WithRuntimeDir(cfgm.Config.RuntimeDir),
				)
				if err != nil {
					plog.Errorf("could not instantiate machine driver for %s: %v", mid.ShortString(), err)
					machinedriver.Observations.Done(mid)
					return
				}

				machinedriver.Drivers[driverType] = driver
			}

			driver := machinedriver.Drivers[driverType]

			if err := driver.Stop(ctx, mid); err != nil {
				plog.Errorf("could not stop machine %s: %v", mid.ShortString(), err)
			} else {
				plog.Infof("stopped %s", mid.ShortString())
			}

			machinedriver.Observations.Done(mid)
		}()
	}

	machinedriver.Observations.Wait()

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

			if machinedriver.Observations.Contains(mid) {
				continue
			}

			plog.Infof("monitoring %s", mid.ShortString())

			var mcfg machine.MachineConfig
			if err := store.LookupMachineConfig(mid, &mcfg); err != nil {
				plog.Errorf("could not look up machine config: %v", err)
				continue
			}

			go func() {
				machinedriver.Observations.Add(mid)

				if args.QuitTogether {
					defer machinedriver.Observations.Done(mid)
				}

				mcfg := &machine.MachineConfig{}
				if err := store.LookupMachineConfig(mid, mcfg); err != nil {
					plog.Errorf("could not look up machine config: %v", err)
					machinedriver.Observations.Done(mid)
					return
				}

				driverType := machinedriver.DriverTypeFromName(mcfg.DriverName)

				if _, ok := machinedriver.Drivers[driverType]; !ok {
					driver, err := machinedriver.New(driverType,
						machinedriveropts.WithLogger(plog),
						machinedriveropts.WithMachineStore(store),
						machinedriveropts.WithRuntimeDir(cfgm.Config.RuntimeDir),
					)
					if err != nil {
						plog.Errorf("could not instantiate machine driver for %s: %v", mid, err)
						machinedriver.Observations.Done(mid)
						return
					}

					machinedriver.Drivers[driverType] = driver
				}

				driver := machinedriver.Drivers[driverType]

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

					machinedriver.Observations.Done(mid)
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
							machinedriver.Observations.Done(mid)
							return
						}

					case err := <-errs:
						if !errors.Is(err, qmp.ErrAcceptedNonEvent) {
							plog.Errorf("%v", err)
						}
						machinedriver.Observations.Done(mid)

					case <-ctx.Done():
						machinedriver.Observations.Done(mid)
						return
					}
				}
			}()
		}

		time.Sleep(time.Second * args.Granularity)
	}

	machinedriver.Observations.Wait()

	return nil
}

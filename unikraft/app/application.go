// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package app

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/xlab/treeprint"
	"gopkg.in/yaml.v3"

	"kraftkit.sh/exec"
	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/yamlmerger"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/schema"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app/volume"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/runtime"
	"kraftkit.sh/unikraft/target"
	"kraftkit.sh/unikraft/template"
)

type Application interface {
	component.Component

	// WorkingDir returns the path to the application's working directory
	WorkingDir() string

	// Unikraft returns the application's unikraft configuration
	Unikraft(context.Context) *core.UnikraftConfig

	// OutDir returns the path to the application's output directory
	OutDir() string

	// Template returns the application's template
	Template() *template.TemplateConfig

	// Runtime returns the application's runtime.
	Runtime() *runtime.Runtime

	// Libraries returns the application libraries' configurations
	Libraries(ctx context.Context) (map[string]*lib.LibraryConfig, error)

	// Targets returns the application's targets
	Targets() []target.Target

	// Rootfs is the desired path containing the filesystem that will be mounted
	// as the root filesystem.  This can either be an initramdisk or a volume.
	Rootfs() string

	// SetRootfs sets the root filesystem path for the application to the given
	// value path.
	SetRootfs(string)

	// Command is the list of arguments passed to the application's runtime.
	Command() []string

	// Extensions returns the application's extensions
	Extensions() component.Extensions

	// Kraftfile returns the application's kraft configuration file
	Kraftfile() *Kraftfile

	// MergeTemplate merges the application's configuration with the given
	// configuration
	MergeTemplate(context.Context, Application) (Application, error)

	// IsConfigured returns a boolean to indicate whether the application has been
	// previously configured.  This is deteremined by finding a non-empty
	// `.config` file within the application's source directory
	IsConfigured(target.Target) bool

	// MakeArgs returns the populated `core.MakeArgs` based on the contents of the
	// instantiated `application`.  This information can be passed directly
	// to Unikraft's build system.
	MakeArgs(context.Context, target.Target) (*core.MakeArgs, error)

	// Make is a method which invokes Unikraft's build system.  You can pass in
	// make options based on the `make` package.  Ultimately, this is an abstract
	// method which will be used by a number of well-known make command goals by
	// Unikraft's build system.
	Make(context.Context, target.Target, ...make.MakeOption) error

	// SyncConfig updates the configuration
	SyncConfig(context.Context, target.Target, ...make.MakeOption) error

	// Configure updates the configuration
	Configure(context.Context, target.Target, kconfig.KeyValueMap, ...make.MakeOption) error

	// Prepare the application
	Prepare(context.Context, target.Target, ...make.MakeOption) error

	// Clean the application
	Clean(context.Context, target.Target, ...make.MakeOption) error

	// Delete the build folder of the application
	Properclean(context.Context, target.Target, ...make.MakeOption) error

	// Fetch component sources for the applications
	Fetch(context.Context, target.Target, ...make.MakeOption) error

	// Set a configuration option for a specific target
	Set(context.Context, target.Target, ...make.MakeOption) error

	// Unset a configuration option for a specific target
	Unset(context.Context, target.Target, ...make.MakeOption) error

	// Build offers an invocation of the Unikraft build system with the contextual
	// information of the application
	Build(context.Context, target.Target, ...BuildOption) error

	// LibraryNames return names for all libraries in this Compose config
	LibraryNames() []string

	// TargetNames return names for all targets in this Compose config
	TargetNames() []string

	// Components returns a unique list of Unikraft components which this
	// applicatiton consists of
	Components(context.Context, ...target.Target) ([]component.Component, error)

	// WithTarget is a reducer that returns the application with only the provided
	// target.
	WithTarget(target.Target) (Application, error)

	// Serialize and save the application to the kraftfile
	Save(context.Context) error

	// Volumes to be used during runtime of an application.
	Volumes() []*volume.VolumeConfig

	// Env variables to be used during building and runtime of application.
	Env() map[string]string

	// Removes library from the project directory
	RemoveLibrary(ctx context.Context, libraryName string) error

	// Add library to the application.
	AddLibrary(context.Context, lib.LibraryConfig) error
}

type application struct {
	name          string
	version       string
	source        string
	path          string
	workingDir    string
	filename      string
	outDir        string
	template      *template.TemplateConfig
	runtime       *runtime.Runtime
	unikraft      *core.UnikraftConfig
	libraries     map[string]*lib.LibraryConfig
	targets       []target.Target
	volumes       []*volume.VolumeConfig
	env           target.Env
	command       []string
	rootfs        string
	kraftfile     *Kraftfile
	configuration kconfig.KeyValueMap
	extensions    component.Extensions
}

func (app *application) Name() string {
	return app.name
}

func (app *application) String() string {
	return app.name
}

func (app *application) Source() string {
	return app.source
}

func (app *application) Version() string {
	return app.version
}

func (app *application) WorkingDir() string {
	return app.workingDir
}

func (app *application) Filename() string {
	return app.filename
}

func (app *application) OutDir() string {
	return app.outDir
}

func (app *application) Template() *template.TemplateConfig {
	return app.template
}

func (app *application) Runtime() *runtime.Runtime {
	return app.runtime
}

func (app *application) Unikraft(ctx context.Context) *core.UnikraftConfig {
	return app.unikraft
}

func (app *application) Libraries(ctx context.Context) (map[string]*lib.LibraryConfig, error) {
	uklibs, err := app.Unikraft(ctx).Libraries(ctx)
	if err != nil {
		return nil, err
	}

	libs := app.libraries

	for _, uklib := range uklibs {
		libs[uklib.Name()] = uklib
	}

	return libs, nil
}

func (app *application) Targets() []target.Target {
	targets := []target.Target{}
	for _, t := range app.targets {
		targets = append(targets, target.Target(t))
	}
	return targets
}

func (app *application) Rootfs() string {
	return app.rootfs
}

func (app *application) SetRootfs(rootfs string) {
	app.rootfs = rootfs
}

func (app *application) Command() []string {
	return app.command
}

func (app *application) Extensions() component.Extensions {
	return app.extensions
}

func (app *application) Kraftfile() *Kraftfile {
	return app.kraftfile
}

func (app *application) MergeTemplate(ctx context.Context, merge Application) (Application, error) {
	if app.name == "" {
		app.name = merge.Name()
	}
	if app.source == "" {
		app.source = merge.Source()
	}
	if app.version == "" {
		app.version = merge.Version()
	}

	// TODO(nderjung): Recursive templates?
	// app.template = merge.Template()

	libs, err := merge.Libraries(ctx)
	if err != nil {
		for id, lib := range libs {
			app.libraries[id] = lib
		}
	}

	// TODO(nderjung): This entire method and procedure needs to be re-thought to
	// be better extensible.  For now, it is unused.  We can safely cast this:
	app.targets = merge.Targets()

	for id, ext := range merge.Extensions() {
		app.extensions[id] = ext
	}

	app.kraftfile = merge.Kraftfile()

	for id, val := range merge.KConfig() {
		app.configuration[id] = val
	}

	// Need to first merge the app configuration over the template
	uk := merge.Unikraft(ctx)
	if app.unikraft != nil {
		uk.KConfig().OverrideBy(app.unikraft.KConfig())
		app.unikraft.KConfig().OverrideBy(uk.KConfig())
	} else {
		app.unikraft, err = core.NewUnikraftFromOptions(
			unikraft.WithContext(ctx, &unikraft.Context{
				UK_NAME:   app.name,
				UK_BASE:   app.workingDir,
				BUILD_DIR: app.outDir,
			}),
			core.WithSource(uk.Source()),
			core.WithKConfig(uk.KConfig()),
			core.WithVersion(uk.Version()),
		)
		if err != nil {
			return nil, err
		}
	}

	return app, nil
}

func (app *application) KConfigTree(ctx context.Context, env ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	var libraryPaths []string

	for _, lib := range app.libraries {
		libraryPaths = append(libraryPaths, lib.Path())
	}

	args := &core.MakeArgs{
		OutputDir:      app.outDir,
		ApplicationDir: app.workingDir,
		LibraryDirs:    strings.Join(libraryPaths, core.MakeDelimeter),
	}

	var buf bytes.Buffer

	m, err := make.NewFromInterface(*args,
		make.WithDirectory(app.unikraft.Path()),
		make.WithNoPrintDirectory(true),
		make.WithTarget("print-vars"),
		make.WithExecOptions(
			exec.WithStdout(bufio.NewWriter(&buf)),
		),
	)
	if err != nil {
		return nil, err
	}

	if err := m.Execute(ctx); err != nil {
		return nil, err
	}

	base := []*kconfig.KeyValue{
		{Key: "UK_BASE", Value: app.unikraft.Path()},
	}

	// Parse each line starting with `[file] `
	for _, line := range strings.Split(buf.String(), "\n") {
		if !strings.HasPrefix(line, "[file] ") {
			continue
		}

		parts := strings.Split(line[7:], " ")

		// Skip make variables
		if strings.HasPrefix(parts[0], ".") {
			continue
		}

		if strings.HasPrefix(parts[0], "_") {
			continue
		}

		// Skip unexported variables
		if strings.ToUpper(parts[0]) != parts[0] {
			continue
		}

		val := strings.Join(parts[2:], " ")
		val = strings.ReplaceAll(val, "//", "/")

		if val == "<recursive>" {
			continue
		}

		base = append(base, &kconfig.KeyValue{Key: parts[0], Value: val})
	}

	return app.unikraft.KConfigTree(ctx, app.KConfig().Override(base...).Slice()...)
}

func (app *application) KConfig() kconfig.KeyValueMap {
	if app.configuration == nil {
		app.configuration = kconfig.KeyValueMap{}
	}

	all := kconfig.KeyValueMap{}

	if app.unikraft != nil {
		all = app.configuration.OverrideBy(app.unikraft.KConfig())
	}

	for _, library := range app.libraries {
		all = all.OverrideBy(library.KConfig())
	}

	return all
}

func (app *application) IsConfigured(tc target.Target) bool {
	f, err := os.Stat(filepath.Join(app.workingDir, tc.ConfigFilename()))
	return err == nil && !f.IsDir() && f.Size() > 0
}

func (app *application) MakeArgs(ctx context.Context, tc target.Target) (*core.MakeArgs, error) {
	components, err := app.Components(ctx, tc)
	if err != nil {
		return nil, fmt.Errorf("could not get application components: %w", err)
	}

	var libraries []string

	// TODO: This is a temporary solution to fix an ordering issue with regard to
	// syscall availability from a libc (which should be included first).  Long-term
	// solution is to determine the library order by generating a DAG via KConfig
	// parsing.
	unformattedLibraries := map[string]*lib.LibraryConfig{}
	for _, c := range components {
		if c.Type() != unikraft.ComponentTypeLib {
			continue
		}

		unformattedLibraries[c.Name()] = c.(*lib.LibraryConfig)
	}

	// Add language libraries that we know require a specific ordering.
	// Currently, these are C++-related libraries. Others may be added as required.
	if unformattedLibraries["libcxxabi"] != nil {
		libraries = append(libraries, unformattedLibraries["libcxxabi"].Path())
		delete(unformattedLibraries, "libcxxabi")
	}
	if unformattedLibraries["libcxx"] != nil {
		libraries = append(libraries, unformattedLibraries["libcxx"].Path())
		delete(unformattedLibraries, "libcxx")
	}
	if unformattedLibraries["libunwind"] != nil {
		libraries = append(libraries, unformattedLibraries["libunwind"].Path())
		delete(unformattedLibraries, "libunwind")
	}
	if unformattedLibraries["compiler-rt"] != nil {
		libraries = append(libraries, unformattedLibraries["compiler-rt"].Path())
		delete(unformattedLibraries, "compiler-rt")
	}
	if unformattedLibraries["libgo"] != nil {
		libraries = append(libraries, unformattedLibraries["libgo"].Path())
		delete(unformattedLibraries, "libgo")
	}

	// All supported libCs right now
	if unformattedLibraries["musl"] != nil {
		libraries = append(libraries, unformattedLibraries["musl"].Path())
		delete(unformattedLibraries, "musl")
	} else if unformattedLibraries["newlib"] != nil {
		libraries = append(libraries, unformattedLibraries["newlib"].Path())
		delete(unformattedLibraries, "newlib")
		if unformattedLibraries["pthread-embedded"] != nil {
			libraries = append(libraries, unformattedLibraries["pthread-embedded"].Path())
			delete(unformattedLibraries, "pthread-embedded")
		}
	}

	orderedLibraries := []string{}
	for _, library := range unformattedLibraries {
		if !library.IsUnpacked() {
			return nil, fmt.Errorf("cannot determine library \"%s\" path without component source", library.Name())
		}

		orderedLibraries = append(orderedLibraries, library.Path())
	}

	slices.Sort(orderedLibraries)
	libraries = append(libraries, orderedLibraries...)

	// TODO: Platforms & architectures

	appDir := app.workingDir
	if app.template != nil {
		appDir = app.template.Path()
	}

	args := &core.MakeArgs{
		OutputDir:      app.outDir,
		ApplicationDir: appDir,
		LibraryDirs:    strings.Join(libraries, core.MakeDelimeter),
	}

	// Set the relevant Unikraft `.config` file when a target is set
	if tc != nil {
		args.ConfigPath = filepath.Join(app.workingDir, tc.ConfigFilename())
	}

	if tc != nil {
		args.Name = tc.Name()
	}

	return args, nil
}

func (app *application) Make(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	mopts = append(mopts,
		make.WithDirectory(app.unikraft.Path()),
		make.WithNoPrintDirectory(true),
		make.WithSyncOutput(true),
	)

	args, err := app.MakeArgs(ctx, tc)
	if err != nil {
		return err
	}

	m, err := make.NewFromInterface(*args, mopts...)
	if err != nil {
		return err
	}

	// Unikraft currently requires each application to have a `Makefile.uk`
	// located within the working directory.  Create it if it does not exist:
	makefile_uk := filepath.Join(app.WorkingDir(), unikraft.Makefile_uk)
	if _, err := os.Stat(makefile_uk); err != nil && os.IsNotExist(err) {
		if _, err := os.OpenFile(makefile_uk, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666); err != nil {
			return fmt.Errorf("could not create application %s: %v", makefile_uk, err)
		}
	}

	return m.Execute(ctx)
}

func (app *application) SyncConfig(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("syncconfig"),
		)...,
	)
}

func (app *application) Configure(ctx context.Context, tc target.Target, extra kconfig.KeyValueMap, mopts ...make.MakeOption) error {
	values := kconfig.KeyValueMap{}
	values.OverrideBy(app.KConfig())

	if tc != nil {
		values.OverrideBy(tc.Architecture().KConfig())
		values.OverrideBy(tc.Platform().KConfig())
		values.OverrideBy(tc.KConfig())

		// This is a special exception used for KraftCloud-centric platform targets.
		if tc.Platform().Name() == "kraftcloud" {
			values.Set("CONFIG_KVM_DEBUG_VGA_CONSOLE", kconfig.No)
			values.Set("CONFIG_KVM_KERNEL_VGA_CONSOLE", kconfig.No)
		}
	}

	if extra != nil {
		values.OverrideBy(extra)
	}

	// Are we embedding an initramfs file into the kernel?
	if values.AnyYes(
		"CONFIG_LIBVFSCORE_FSTAB", // Deprecated
		"CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD",
		"CONFIG_LIBVFSCORE_AUTOMOUNT_CI_EINITRD",
	) {
		// If the user has not specified a path, we set one specifically which
		// is specific to the target.
		if val, exists := values.Get("CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD_PATH"); !exists || val.Value == "" {
			values.Set("CONFIG_LIBVFSCORE_AUTOMOUNT_EINITRD_PATH",
				filepath.Join(
					app.outDir,
					fmt.Sprintf(initrd.DefaultInitramfsArchFileName, tc.Architecture().String()),
				),
			)
		}
	}

	for _, kv := range values {
		log.G(ctx).WithFields(logrus.Fields{
			kv.Key: kv.Value,
		}).Debugf("defconfig")
	}

	// Write the configuration to a temporary file
	tmpfile, err := os.CreateTemp("", app.Name()+"-config*")
	if err != nil {
		return fmt.Errorf("could not create temporary defconfig file: %v", err)
	}

	defer tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	// Save and sync the file to the temporary file
	if _, err := tmpfile.Write([]byte(values.String())); err != nil {
		return err
	}
	if err := tmpfile.Sync(); err != nil {
		return err
	}

	// TODO: This make dependency should be upstreamed into the Unikraft core as a
	// dependency of `make defconfig`
	if err := app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget(fmt.Sprintf("%s/Makefile", app.outDir)),
			make.WithProgressFunc(nil),
		)...,
	); err != nil {
		return err
	}

	if err := app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("defconfig"),
			make.WithVar("UK_DEFCONFIG", tmpfile.Name()),
		)...,
	); err != nil {
		return fmt.Errorf("configuring: %w", err)
	}

	// Post-process the target by parsing any available .config file which will be
	// the result artifact from the previously invoked make command.
	kvmap, err := kconfig.NewKeyValueMapFromFile(
		filepath.Join(app.workingDir, tc.ConfigFilename()),
	)
	if err != nil {
		return fmt.Errorf("processing KConfig lock file: %w", err)
	}

	tc.KConfig().OverrideBy(kvmap)

	return nil
}

func (app *application) Prepare(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("prepare"),
		)...,
	)
}

func (app *application) Clean(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("clean"),
		)...,
	)
}

func (app *application) Properclean(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("properclean"),
		)...,
	)
}

func (app *application) Fetch(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("fetch"),
		)...,
	)
}

func (app *application) Set(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	// Write the configuration to a temporary file
	// tmpfile, err := ioutil.TempFile("", app.Name()+"-config*")
	// if err != nil {
	// 	return err
	// }
	// defer tmpfile.Close()
	// defer os.Remove(tmpfile.Name())

	// // Save and sync the config file
	// tmpfile.WriteString(app.Configuration.String())
	// tmpfile.Sync()

	// // Give the file to the make command to import
	// mopts = append(mopts,
	// 	make.WithExecOptions(
	// 		exec.WithEnvKey(unikraft.UK_DEFCONFIG, tmpfile.Name()),
	// 	),
	// )

	// return app.Configure(mopts...)

	return nil
}

func (app *application) Unset(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	// // Write the configuration to a temporary file
	// tmpfile, err := ioutil.TempFile("", app.Name()+"-config*")
	// if err != nil {
	// 	return err
	// }
	// defer tmpfile.Close()
	// defer os.Remove(tmpfile.Name())

	// // Save and sync the config file
	// tmpfile.WriteString(app.Configuration.String())
	// tmpfile.Sync()

	// // Give the file to the make command to import
	// mopts = append(mopts,
	// 	make.WithExecOptions(
	// 		exec.WithEnvKey(unikraft.UK_DEFCONFIG, tmpfile.Name()),
	// 	),
	// )

	// return app.Configure(mopts...)

	return nil
}

// Build offers an invocation of the Unikraft build system with the contextual
// information of the applications
func (app *application) Build(ctx context.Context, tc target.Target, opts ...BuildOption) error {
	bopts := &BuildOptions{}
	for _, o := range opts {
		err := o(bopts)
		if err != nil {
			return fmt.Errorf("could not apply build option: %v", err)
		}
	}

	if !app.unikraft.IsUnpacked() {
		// TODO: Produce better error messages (see #34).  In this case, we should
		// indicate that `kraft pkg pull` needs to occur
		return fmt.Errorf("cannot build without Unikraft core component source")
	}

	mopts := []make.MakeOption{
		make.WithProgressFunc(bopts.onProgress),
	}

	// This is a special exception used for KraftCloud-centric platform targets.
	// This includes using the ability to rename the kernal image to represent
	// this as a platform (see [0] for additional details) and setting specific
	// KConfig options.
	//
	// [0]: https://github.com/unikraft/unikraft/pull/1169
	if tc.Platform().Name() == "kraftcloud" {
		mopts = append(mopts, make.WithVar("UK_IMAGE_NAME_OVERWRITE", fmt.Sprintf("%s_kraftcloud-%s", tc.Name(), tc.Architecture().Name())))
	}

	bopts.mopts = append(bopts.mopts, mopts...)

	if !bopts.noPrepare {
		if err := app.Prepare(
			ctx,
			tc,
			append(
				bopts.mopts,
				make.WithProgressFunc(nil),
			)...); err != nil {
			return err
		}
	}

	return app.Make(ctx, tc, bopts.mopts...)
}

// LibraryNames return names for all libraries in this Compose config
func (app *application) LibraryNames() []string {
	var names []string
	for k := range app.libraries {
		names = append(names, k)
	}

	sort.Strings(names)

	return names
}

// TargetNames return names for all targets in this Compose config
func (app *application) TargetNames() []string {
	var names []string
	for _, k := range app.targets {
		names = append(names, k.Name())
	}

	sort.Strings(names)

	return names
}

// Components returns a unique list of Unikraft components which this
// applicatiton consists of
func (app *application) Components(ctx context.Context, targets ...target.Target) ([]component.Component, error) {
	components := []component.Component{}

	if unikraft := app.Unikraft(ctx); unikraft != nil {
		components = append(components, unikraft)
	}

	if app.template != nil && len(app.template.Path()) > 0 {
		template, err := NewApplicationFromOptions(
			WithWorkingDir(app.template.Path()),
		)
		if err != nil {
			return nil, fmt.Errorf("could not read template application: %w", err)
		}

		templateComponents, err := template.Components(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not get template components: %w", err)
		}

		components = append(components, templateComponents...)
	}

	for _, library := range app.libraries {
		components = append(components, library)
	}

	// Add KraftCloud-specific libraries when a target with this name is provided.
	var ukp *lib.LibraryConfig
	for _, targ := range targets {
		if targ.Platform().String() != "kraftcloud" {
			continue
		}

		// If the user has already added a library called `ukp`, do not proceed.
		if _, ok := app.libraries["ukp"]; ok {
			continue
		}

		// If the user has already added a library called `ukp-bin`, do not proceed.
		if _, ok := app.libraries["ukp-bin"]; ok {
			continue
		}

		if ukp == nil {
			ctx = unikraft.WithContext(ctx, &unikraft.Context{
				UK_NAME:   app.name,
				UK_BASE:   app.workingDir,
				BUILD_DIR: app.outDir,
			})
			lukp, err := lib.TransformFromSchema(ctx, "ukp-bin", map[string]interface{}{
				"source":  "https://github.com/unikraft-io/lib-ukp-bin.git",
				"version": "stable",
			})
			if err != nil {
				return nil, fmt.Errorf("could not add kraftcloud internal libraries: %w", err)
			}
			ukp = &lukp

			// Add relevant KConfig option
			targ.KConfig().Set("CONFIG_LIBUKP", kconfig.Yes)
		}

		components = append(components, ukp)
	}

	// TODO: Get unique components from each target.  A target will contain at
	// least two components: the architecture and the platform.  Both of these
	// components can stem from the Unikraft core (in the case of built-in
	// architectures and components).
	// for _, targ := range app.Targets {
	// 	components = append(components, targ)
	// }

	return components, nil
}

func (app *application) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

func (app *application) Path() string {
	return app.workingDir
}

func (app *application) PrintInfo(ctx context.Context) string {
	tree := treeprint.NewWithRoot(component.NameAndVersion(app))

	uk := tree.AddBranch(component.NameAndVersion(app.unikraft))
	uklibs, err := app.unikraft.Libraries(ctx)
	if err == nil {
		for _, uklib := range uklibs {
			uk.AddNode(uklib.Name())
		}
	}

	if len(app.libraries) > 0 {
		libraries := tree.AddBranch(fmt.Sprintf("libraries (%d)", len(app.libraries)))
		for _, library := range app.libraries {
			libraries.AddNode(component.NameAndVersion(library))
		}
	}

	if len(app.targets) > 0 {
		targets := tree.AddBranch(fmt.Sprintf("targets (%d)", len(app.targets)))
		for _, t := range app.targets {
			branch := targets.AddBranch(component.NameAndVersion(t))
			branch.AddNode(fmt.Sprintf("architecture: %s", component.NameAndVersion(t.Architecture())))
			branch.AddNode(fmt.Sprintf("platform:     %s", component.NameAndVersion(t.Platform())))
		}
	}

	return tree.String()
}

func (app *application) WithTarget(targ target.Target) (Application, error) {
	ret := app
	ret.targets = []target.Target{targ}
	return ret, nil
}

// MarshalYAML makes application implement yaml.Marshaller
func (app *application) MarshalYAML() (interface{}, error) {
	ret := map[string]interface{}{
		"name":     app.name,
		"unikraft": app.unikraft,
	}

	if app.kraftfile.config["specification"] != nil {
		ret["specification"] = app.kraftfile.config["specification"]
	} else if app.kraftfile.config["spec"] != nil {
		ret["spec"] = app.kraftfile.config["spec"]
	} else {
		ret["spec"] = schema.SchemaVersionLatest
	}

	// We purposefully do not marshal the configuration as this top level
	// kconfig is redundant with the kconfig files in the libraries and
	// unikraft.
	// TODO: Figure out if anything is actually needed at this top level
	// if app.configuration != nil && len(app.configuration) > 0 {
	// 	ret["kconfig"] = app.configuration
	// }

	if app.targets != nil && len(app.targets) > 0 {
		ret["targets"] = app.targets
	}

	if app.libraries != nil && len(app.libraries) > 0 {
		ret["libraries"] = app.libraries
	}

	if app.template != nil {
		ret["template"] = app.template
	}

	if len(app.rootfs) > 0 {
		ret["rootfs"] = app.rootfs
	}

	if len(app.command) > 0 {
		ret["cmd"] = app.command
	}

	if app.volumes != nil && len(app.volumes) > 0 {
		ret["volumes"] = app.volumes
	}

	if app.runtime != nil {
		ret["runtime"] = app.runtime
	}

	return ret, nil
}

func mergeOverExistingKraftfile(ctx context.Context, app Application) error {
	// Marshal the app object to YAML
	yamlData, err := yaml.Marshal(app)
	if err != nil {
		return err
	}

	// Validate the YAML data against the schema
	var yamlMap map[string]interface{}
	err = yaml.Unmarshal(yamlData, &yamlMap)
	if err != nil {
		return err
	}
	err = schema.Validate(ctx, yamlMap)
	if err != nil {
		return err
	}

	yamlFile, err := os.ReadFile(app.Kraftfile().path)
	if err != nil {
		return err
	}
	var into yaml.Node
	err = yaml.Unmarshal(yamlFile, &into)
	if err != nil {
		return err
	}

	var from yaml.Node
	if err := yaml.Unmarshal(yamlData, &from); err != nil {
		return fmt.Errorf("could not unmarshal YAML: %s", err)
	}

	if err := yamlmerger.RecursiveMerge(&from, &into); err != nil {
		return fmt.Errorf("could not merge YAML: %s", err)
	}

	// Marshal the Node structure back to YAML
	outYaml, err := yaml.Marshal(&into)
	if err != nil {
		return err
	}

	// Write the YAML data to the file
	err = os.WriteFile(app.Kraftfile().path, outYaml, 0o644)
	if err != nil {
		return err
	}

	return nil
}

// Marshalling the application does not allow us to impose the order of the fields
// in the yaml file. This is generally not a problem as updating the yaml file will
// preserve the old order, but in the case we are generating a fresh yaml file, we
// need to do something hacky to ensure the order is correct.
func saveNewKraftfile(ctx context.Context, app Application) error {
	// Open the kraftfile for writing
	kraftfile, err := os.OpenFile(app.Kraftfile().path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer kraftfile.Close()

	// Write the schema version to the file by checking in existing Kraftfile
	if app.Kraftfile().config["specification"] != nil {
		_, err = kraftfile.WriteString(fmt.Sprintf("specification: '%s'\n", app.Kraftfile().config["specification"]))
	} else if app.Kraftfile().config["spec"] != nil {
		_, err = kraftfile.WriteString(fmt.Sprintf("spec: '%s'\n", app.Kraftfile().config["spec"]))
	} else {
		_, err = kraftfile.WriteString(fmt.Sprintf("spec: '%s'\n", schema.SchemaVersionLatest))
	}
	if err != nil {
		return err
	}

	// Hacky way to assure the order of fields in the kraftfile
	ukernel := app.Unikraft(ctx)
	coreApp, err := NewApplicationFromOptions(
		WithName(app.Name()),
		WithKraftfile(app.Kraftfile()),
		WithUnikraft(ukernel),
		WithRuntime(app.Runtime()),
		WithTemplate(app.Template()),
	)
	if err != nil {
		return err
	}

	if err = mergeOverExistingKraftfile(ctx, coreApp); err != nil {
		return err
	}

	if err = mergeOverExistingKraftfile(ctx, app); err != nil {
		return err
	}

	return nil
}

func (app *application) Save(ctx context.Context) error {
	// Open the kratfile for reading
	kraftfile, err := os.ReadFile(app.kraftfile.path)

	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return saveNewKraftfile(ctx, app)
	} else if len(kraftfile) == 0 {
		return saveNewKraftfile(ctx, app)
	}
	return mergeOverExistingKraftfile(ctx, app)
}

// Volumes implemenets Application.
func (app *application) Volumes() []*volume.VolumeConfig {
	return app.volumes
}

// Env implements Application
func (app *application) Env() map[string]string {
	return app.env
}

func (app *application) RemoveLibrary(ctx context.Context, libraryName string) error {
	isLibraryExistInProject := false
	for libKey, lib := range app.libraries {
		if lib.Name() == libraryName {
			isLibraryExistInProject = true
			delete(app.libraries, libKey)

			yamlFile, err := os.ReadFile(app.kraftfile.path)
			if err != nil {
				return err
			}

			yamlMap := app
			err = yaml.Unmarshal(yamlFile, &yamlMap)
			if err != nil {
				return err
			}

			delete(yamlMap.libraries, libKey)
			err = saveNewKraftfile(ctx, yamlMap)
			if err != nil {
				return err
			}

			// Remove library directory from the project directory
			libPath := filepath.Join(app.WorkingDir(), unikraft.LibsDir, libraryName)
			if _, err = os.Stat(libPath); err == nil {
				err = os.RemoveAll(libPath)
				if err != nil {
					return err
				}
			}
		}
	}
	if !isLibraryExistInProject {
		return fmt.Errorf("library %s does not exist in the project", libraryName)
	}
	return nil
}

func (app *application) AddLibrary(ctx context.Context, library lib.LibraryConfig) error {
	if app.libraries == nil {
		app.libraries = map[string]*lib.LibraryConfig{}
	}
	app.libraries[library.Name()] = &library

	return nil
}

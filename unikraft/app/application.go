// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/xlab/treeprint"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/make"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/target"
	"kraftkit.sh/unikraft/template"
)

type Application interface {
	component.Component

	// WorkingDir returns the path to the application's working directory
	WorkingDir() string

	// Unikraft returns the application's unikraft configuration
	Unikraft(context.Context) core.Unikraft

	// OutDir returns the path to the application's output directory
	OutDir() string

	// Template returns the application's template
	Template() template.Template

	// Libraries returns the application libraries' configurations
	Libraries(ctx context.Context) (lib.Libraries, error)

	// Targets returns the application's targets
	Targets() target.Targets

	// Extensions returns the application's extensions
	Extensions() component.Extensions

	// Kraftfiles returns the application's kraft configuration files
	Kraftfiles() []string

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
	MakeArgs(target.Target) (*core.MakeArgs, error)

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

	// TargetByName returns the `target.Target` based on an input name
	TargetByName(string) (target.Target, error)

	// Components returns a unique list of Unikraft components which this
	// applicatiton consists of
	Components(context.Context) ([]component.Component, error)

	// WithTarget is a reducer that returns the application with only the provided
	// target.
	WithTarget(target.Target) (Application, error)
}

type application struct {
	name          string
	version       string
	source        string
	path          string
	workingDir    string
	filename      string
	outDir        string
	template      template.TemplateConfig
	unikraft      core.UnikraftConfig
	libraries     lib.Libraries
	targets       target.Targets
	kraftfiles    []string
	configuration kconfig.KeyValueMap
	extensions    component.Extensions
}

func (app application) Name() string {
	return app.name
}

func (app application) Source() string {
	return app.source
}

func (app application) Version() string {
	return app.version
}

func (app application) String() string {
	return unikraft.TypeNameVersion(app)
}

func (app application) WorkingDir() string {
	return app.workingDir
}

func (app application) Filename() string {
	return app.filename
}

func (app application) OutDir() string {
	return app.outDir
}

func (app application) Template() template.Template {
	return app.template
}

func (app application) Unikraft(ctx context.Context) core.Unikraft {
	return app.unikraft
}

func (app application) Libraries(ctx context.Context) (lib.Libraries, error) {
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

func (app application) Targets() target.Targets {
	return app.targets
}

func (app application) Extensions() component.Extensions {
	return app.extensions
}

func (app application) Kraftfiles() []string {
	return app.kraftfiles
}

func (app application) MergeTemplate(ctx context.Context, merge Application) (Application, error) {
	app.name = merge.Name()
	app.source = merge.Source()
	app.version = merge.Version()
	app.path = merge.Path()
	app.workingDir = merge.WorkingDir()
	app.outDir = merge.OutDir()
	app.template = merge.Template().(template.TemplateConfig)

	libs, err := merge.Libraries(ctx)
	if err != nil {
		for id, lib := range libs {
			app.libraries[id] = lib
		}
	}

	app.targets = merge.Targets()

	for id, ext := range merge.Extensions() {
		app.extensions[id] = ext
	}

	app.kraftfiles = append(app.kraftfiles, merge.Kraftfiles()...)

	for id, val := range merge.KConfig() {
		app.configuration[id] = val
	}

	// Need to first merge the app configuration over the template
	uk := merge.Unikraft(ctx)
	uk.KConfig().OverrideBy(app.unikraft.KConfig())
	app.unikraft = uk.(core.UnikraftConfig)

	return app, nil
}

func (app application) KConfigTree(env ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	config_uk := filepath.Join(app.workingDir, unikraft.Config_uk)
	if _, err := os.Stat(config_uk); err != nil {
		return nil, fmt.Errorf("could not read component Config.uk: %v", err)
	}

	return kconfig.Parse(config_uk, app.KConfig().Override(env...).Slice()...)
}

func (app application) KConfig() kconfig.KeyValueMap {
	if app.configuration == nil {
		app.configuration = kconfig.KeyValueMap{}
	}

	all := app.configuration.OverrideBy(app.unikraft.KConfig())

	for _, library := range app.libraries {
		all = all.OverrideBy(library.KConfig())
	}

	return all
}

func (app application) IsConfigured(tc target.Target) bool {
	f, err := os.Stat(filepath.Join(app.workingDir, tc.ConfigFilename()))
	return err == nil && !f.IsDir() && f.Size() > 0
}

func (app application) MakeArgs(tc target.Target) (*core.MakeArgs, error) {
	var libraries []string

	// TODO: This is a temporary solution to fix an ordering issue with regard to
	// syscall availability from a libc (which should be included first).  Long-term
	// solution is to determine the library order by generating a DAG via KConfig
	// parsing.
	unformattedLibraries := lib.Libraries{}
	for k, v := range app.libraries {
		unformattedLibraries[k] = v
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

	for _, library := range unformattedLibraries {
		if !library.IsUnpacked() {
			return nil, fmt.Errorf("cannot determine library \"%s\" path without component source", library.Name())
		}

		libraries = append(libraries, library.Path())
	}

	// TODO: Platforms & architectures

	args := &core.MakeArgs{
		OutputDir:      app.outDir,
		ApplicationDir: app.workingDir,
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

func (app application) Make(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	mopts = append(mopts,
		make.WithDirectory(app.unikraft.Path()),
		make.WithNoPrintDirectory(true),
	)

	args, err := app.MakeArgs(tc)
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

func (app application) SyncConfig(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("syncconfig"),
		)...,
	)
}

func (app application) Configure(ctx context.Context, tc target.Target, extra kconfig.KeyValueMap, mopts ...make.MakeOption) error {
	values := kconfig.KeyValueMap{}
	values.OverrideBy(app.KConfig())

	if tc != nil {
		values.OverrideBy(tc.KConfig())
	}

	if extra != nil {
		values.OverrideBy(extra)
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

	return app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("defconfig"),
			make.WithVar("UK_DEFCONFIG", tmpfile.Name()),
		)...,
	)
}

func (app application) Prepare(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("prepare"),
		)...,
	)
}

func (app application) Clean(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("clean"),
		)...,
	)
}

func (app application) Properclean(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("properclean"),
		)...,
	)
}

func (app application) Fetch(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
	return app.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("fetch"),
		)...,
	)
}

func (app application) Set(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
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

func (app application) Unset(ctx context.Context, tc target.Target, mopts ...make.MakeOption) error {
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
func (app application) Build(ctx context.Context, tc target.Target, opts ...BuildOption) error {
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

	bopts.mopts = append(bopts.mopts, []make.MakeOption{
		make.WithProgressFunc(bopts.onProgress),
	}...)

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
func (app application) LibraryNames() []string {
	var names []string
	for k := range app.libraries {
		names = append(names, k)
	}

	sort.Strings(names)

	return names
}

// TargetNames return names for all targets in this Compose config
func (app application) TargetNames() []string {
	var names []string
	for _, k := range app.targets {
		names = append(names, k.Name())
	}

	sort.Strings(names)

	return names
}

// TargetByName returns the `*target.TargetConfig` based on an input name
func (app application) TargetByName(name string) (target.Target, error) {
	if len(name) == 0 {
		return nil, fmt.Errorf("no target name specified in lookup")
	}

	for _, k := range app.targets {
		if k.Name() == name {
			return k, nil
		}
	}

	return nil, fmt.Errorf("unknown target: %s", name)
}

// Components returns a unique list of Unikraft components which this
// applicatiton consists of
func (app application) Components(ctx context.Context) ([]component.Component, error) {
	components := []component.Component{
		app.Unikraft(ctx),
	}

	if app.template.Name() != "" {
		components = append(components, app.template)
	}

	for _, library := range app.libraries {
		components = append(components, library)
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

func (app application) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

func (app application) Path() string {
	return app.workingDir
}

func (app application) PrintInfo(ctx context.Context) string {
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

func (app application) WithTarget(targ target.Target) (Application, error) {
	ret := app
	ret.targets = target.Targets{targ.(*target.TargetConfig)}
	return ret, nil
}

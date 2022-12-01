// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package app

import (
	"context"
	"fmt"
	"io/ioutil"
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
}

type ApplicationConfig struct {
	component.ComponentConfig

	workingDir    string                  `yaml:"-" json:"-"`
	filename      string                  `yaml:"-" json:"-"`
	outDir        string                  `yaml:",omitempty"`
	template      template.TemplateConfig `yaml:",omitempty"`
	unikraft      core.UnikraftConfig     `yaml:",omitempty"`
	libraries     lib.Libraries           `yaml:",omitempty"`
	targets       target.Targets          `yaml:",omitempty"`
	extensions    component.Extensions    `yaml:",inline" json:"-"` // https://github.com/golang/go/issues/6213
	kraftFiles    []string                `yaml:"-" json:"-"`
	configuration kconfig.KConfigValues   `yaml:"-" json:"-"`
}

func (ac ApplicationConfig) Name() string {
	return ac.ComponentConfig.Name
}

func (ac ApplicationConfig) Source() string {
	return ac.ComponentConfig.Source
}

func (ac ApplicationConfig) Version() string {
	return ac.ComponentConfig.Version
}

func (ac ApplicationConfig) Component() component.ComponentConfig {
	return ac.ComponentConfig
}

// WorkingDir returns the path to the application's working directory
func (ac ApplicationConfig) WorkingDir() string {
	return ac.workingDir
}

// Filename returns the path to the application's executable
func (ac ApplicationConfig) Filename() string {
	return ac.filename
}

// OutDir returns the path to the application's output directory
func (ac ApplicationConfig) OutDir() string {
	return ac.outDir
}

// Template returns the application's template
func (ac ApplicationConfig) Template() template.TemplateConfig {
	return ac.template
}

// Unikraft returns the application's unikraft configuration
func (ac ApplicationConfig) Unikraft() (core.UnikraftConfig, error) {
	if ac.template.Source() != "" && !ac.template.IsUnpackedInProject() {
		return core.UnikraftConfig{}, fmt.Errorf("Unikraft(): template source is not unpacked in project")
	}

	return ac.unikraft, nil
}

// Libraries returns the application libraries' configurations
func (ac ApplicationConfig) Libraries() (lib.Libraries, error) {
	if ac.template.Source() != "" && !ac.template.IsUnpackedInProject() {
		return lib.Libraries{}, fmt.Errorf("Libraries(): template source is not unpacked in project")
	}

	return ac.libraries, nil
}

// Targets returns the application's targets
func (ac ApplicationConfig) Targets() (target.Targets, error) {
	if ac.template.Source() != "" && !ac.template.IsUnpackedInProject() {
		return target.Targets{}, fmt.Errorf("Targets(): template source is not unpacked in project")
	}

	return ac.targets, nil
}

// Extensions returns the application's extensions
func (ac ApplicationConfig) Extensions() (component.Extensions, error) {
	if ac.template.Source() != "" && !ac.template.IsUnpackedInProject() {
		return component.Extensions{}, fmt.Errorf("Extensions(): template source is not unpacked in project")
	}

	return ac.extensions, nil
}

// KraftFiles returns the application's kraft configuration files
func (ac ApplicationConfig) KraftFiles() ([]string, error) {
	if ac.template.Source() != "" && !ac.template.IsUnpackedInProject() {
		return []string{}, fmt.Errorf("KraftFiles(): template source is not unpacked in project")
	}

	return ac.kraftFiles, nil
}

// Configuration returns the application's kconfig list
func (ac ApplicationConfig) Configuration() (kconfig.KConfigValues, error) {
	if ac.template.Source() != "" && !ac.template.IsUnpackedInProject() {
		return kconfig.KConfigValues{}, fmt.Errorf("Configuration(): template source is not unpacked in project")
	}

	return ac.configuration, nil
}

// MergeTemplate merges the application's configuration with the given
// configuration
func (ac *ApplicationConfig) MergeTemplate(app *ApplicationConfig) *ApplicationConfig {
	ac.ComponentConfig = app.ComponentConfig

	ac.workingDir = app.workingDir
	ac.filename = app.filename
	ac.outDir = app.outDir
	ac.template = app.template

	// Change all workdirs
	for i := range ac.libraries {
		lib := ac.libraries[i]
		lib.SetWorkdir(ac.workingDir)
		ac.libraries[i] = lib
	}

	for id, lib := range app.libraries {
		ac.libraries[id] = lib
	}

	ac.targets = app.targets

	for id, ext := range app.extensions {
		ac.extensions[id] = ext
	}

	ac.kraftFiles = append(ac.kraftFiles, app.kraftFiles...)

	for id, val := range app.configuration {
		ac.configuration[id] = val
	}

	// Need to first merge the app configuration over the template
	uk := app.unikraft
	uk.Configuration = ac.unikraft.Configuration
	for id, val := range app.unikraft.Configuration {
		uk.Configuration[id] = val
	}
	ac.unikraft = uk

	return ac
}

func (ac ApplicationConfig) KConfigMenu() (*kconfig.KConfigFile, error) {
	config_uk := filepath.Join(ac.workingDir, unikraft.Config_uk)
	if _, err := os.Stat(config_uk); err != nil {
		return nil, fmt.Errorf("could not read component Config.uk: %v", err)
	}

	return kconfig.Parse(config_uk)
}

func (ac ApplicationConfig) KConfigValues() (kconfig.KConfigValues, error) {
	vAll := kconfig.KConfigValues{}

	vCore, err := ac.unikraft.KConfigValues()
	if err != nil {
		return nil, fmt.Errorf("could not read Unikraft core KConfig values: %v", err)
	}

	vAll.OverrideBy(vCore)

	for _, library := range ac.libraries {
		vLib, err := library.KConfigValues()
		if err != nil {
			return nil, fmt.Errorf("could not %s's KConfig values: %v", library.Name(), err)
		}

		vAll.OverrideBy(vLib)
	}

	return vAll, nil
}

// KConfigFile returns the path to the application's .config file or the
// target-specific `.config` file which is formatted `.config.<TARGET-NAME>`
func (ac *ApplicationConfig) KConfigFile(tc *target.TargetConfig) string {
	k := filepath.Join(ac.workingDir, kconfig.DotConfigFileName)

	if tc != nil {
		k += "." + tc.Name()
	}

	return k
}

// IsConfigured returns a boolean to indicate whether the application has been
// previously configured.  This is deteremined by finding a non-empty `.config`
// file within the application's source directory
func (ac *ApplicationConfig) IsConfigured(tc *target.TargetConfig) bool {
	f, err := os.Stat(ac.KConfigFile(tc))
	return err == nil && !f.IsDir() && f.Size() > 0
}

// MakeArgs returns the populated `core.MakeArgs` based on the contents of the
// instantiated `ApplicationConfig`.  This information can be passed directly to
// Unikraft's build system.
func (a *ApplicationConfig) MakeArgs(tc *target.TargetConfig) (*core.MakeArgs, error) {
	var libraries []string

	for _, library := range a.libraries {
		if !library.IsUnpackedInProject() {
			return nil, fmt.Errorf("cannot determine library \"%s\" path without component source", library.Name())
		}

		src, err := library.SourceDir()
		if err != nil {
			return nil, err
		}

		libraries = append(libraries, src)
	}

	// TODO: Platforms & architectures

	args := &core.MakeArgs{
		OutputDir:      a.outDir,
		ApplicationDir: a.workingDir,
		LibraryDirs:    strings.Join(libraries, core.MakeDelimeter),
		ConfigPath:     a.KConfigFile(tc),
	}

	if tc != nil {
		args.Name = tc.Name()
	}

	return args, nil
}

// Make is a method which invokes Unikraft's build system.  You can pass in make
// options based on the `make` package.  Ultimately, this is an abstract method
// which will be used by a number of well-known make command goals by Unikraft's
// build system.
func (a *ApplicationConfig) Make(ctx context.Context, tc *target.TargetConfig, mopts ...make.MakeOption) error {
	coreSrc, err := a.unikraft.SourceDir()
	if err != nil {
		return err
	}

	mopts = append(mopts,
		make.WithDirectory(coreSrc),
		make.WithNoPrintDirectory(true),
	)

	args, err := a.MakeArgs(tc)
	if err != nil {
		return err
	}

	m, err := make.NewFromInterface(*args, mopts...)
	if err != nil {
		return err
	}

	// Unikraft currently requires each application to have a `Makefile.uk`
	// located within the working directory.  Create it if it does not exist:
	makefile_uk := filepath.Join(a.WorkingDir(), unikraft.Makefile_uk)
	if _, err := os.Stat(makefile_uk); err != nil && os.IsNotExist(err) {
		if _, err := os.OpenFile(makefile_uk, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666); err != nil {
			return fmt.Errorf("could not create application %s: %v", makefile_uk, err)
		}
	}

	return m.Execute(ctx)
}

// SyncConfig updates the configuration
func (a *ApplicationConfig) SyncConfig(ctx context.Context, tc *target.TargetConfig, mopts ...make.MakeOption) error {
	return a.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("syncconfig"),
		)...,
	)
}

// Defconfig updates the configuration
func (ac *ApplicationConfig) DefConfig(ctx context.Context, tc *target.TargetConfig, extra *kconfig.KConfigValues, mopts ...make.MakeOption) error {
	appk, err := ac.KConfigValues()
	if err != nil {
		return fmt.Errorf("could not read application KConfig values: %v", err)
	}

	values := kconfig.KConfigValues{}
	values.OverrideBy(appk)

	if tc != nil {
		targk, err := tc.KConfigValues()
		if err != nil {
			return fmt.Errorf("could not read target KConfig values: %v", err)
		}

		values.OverrideBy(targk)
	}

	if extra != nil {
		values.OverrideBy(*extra)
	}

	for _, v := range values {
		log.G(ctx).WithFields(logrus.Fields{
			v.Name: v.Value,
		}).Debugf("defconfig")
	}

	// Write the configuration to a temporary file
	tmpfile, err := ioutil.TempFile("", ac.Name()+"-config*")
	if err != nil {
		return fmt.Errorf("could not create temporary defconfig file: %v", err)
	}

	defer tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	// Save and sync the file to the temporary file
	tmpfile.Write([]byte(values.String()))
	tmpfile.Sync()

	// TODO: This make dependency should be upstreamed into the Unikraft core as a
	// dependency of `make defconfig`
	if err := ac.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget(fmt.Sprintf("%s/Makefile", ac.outDir)),
			make.WithProgressFunc(nil),
		)...,
	); err != nil {
		return err
	}

	return ac.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("defconfig"),
			make.WithVar("UK_DEFCONFIG", tmpfile.Name()),
		)...,
	)
}

// Configure the application
func (a *ApplicationConfig) Configure(ctx context.Context, tc *target.TargetConfig, mopts ...make.MakeOption) error {
	return a.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("configure"),
		)...,
	)
}

// Prepare the application
func (a *ApplicationConfig) Prepare(ctx context.Context, tc *target.TargetConfig, mopts ...make.MakeOption) error {
	return a.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("prepare"),
		)...,
	)
}

// Clean the application
func (a *ApplicationConfig) Clean(ctx context.Context, tc *target.TargetConfig, mopts ...make.MakeOption) error {
	return a.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("clean"),
		)...,
	)
}

// Delete the build folder of the application
func (a *ApplicationConfig) Properclean(ctx context.Context, tc *target.TargetConfig, mopts ...make.MakeOption) error {
	return a.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("properclean"),
		)...,
	)
}

// Fetch component sources for the applications
func (a *ApplicationConfig) Fetch(ctx context.Context, tc *target.TargetConfig, mopts ...make.MakeOption) error {
	return a.Make(
		ctx,
		tc,
		append(mopts,
			make.WithTarget("fetch"),
		)...,
	)
}

func (a *ApplicationConfig) Set(ctx context.Context, tc *target.TargetConfig, mopts ...make.MakeOption) error {
	// Write the configuration to a temporary file
	// tmpfile, err := ioutil.TempFile("", a.Name()+"-config*")
	// if err != nil {
	// 	return err
	// }
	// defer tmpfile.Close()
	// defer os.Remove(tmpfile.Name())

	// // Save and sync the config file
	// tmpfile.WriteString(a.Configuration.String())
	// tmpfile.Sync()

	// // Give the file to the make command to import
	// mopts = append(mopts,
	// 	make.WithExecOptions(
	// 		exec.WithEnvKey(unikraft.UK_DEFCONFIG, tmpfile.Name()),
	// 	),
	// )

	// return a.DefConfig(mopts...)

	return nil
}

func (a *ApplicationConfig) Unset(ctx context.Context, tc *target.TargetConfig, mopts ...make.MakeOption) error {
	// // Write the configuration to a temporary file
	// tmpfile, err := ioutil.TempFile("", a.Name()+"-config*")
	// if err != nil {
	// 	return err
	// }
	// defer tmpfile.Close()
	// defer os.Remove(tmpfile.Name())

	// // Save and sync the config file
	// tmpfile.WriteString(a.Configuration.String())
	// tmpfile.Sync()

	// // Give the file to the make command to import
	// mopts = append(mopts,
	// 	make.WithExecOptions(
	// 		exec.WithEnvKey(unikraft.UK_DEFCONFIG, tmpfile.Name()),
	// 	),
	// )

	// return a.DefConfig(mopts...)

	return nil
}

// Build offers an invocation of the Unikraft build system with the contextual
// information of the ApplicationConfigs
func (a *ApplicationConfig) Build(ctx context.Context, tc *target.TargetConfig, opts ...BuildOption) error {
	bopts := &BuildOptions{}
	for _, o := range opts {
		err := o(bopts)
		if err != nil {
			return fmt.Errorf("could not apply build option: %v", err)
		}
	}

	if !a.unikraft.IsUnpackedInProject() {
		// TODO: Produce better error messages (see #34).  In this case, we should
		// indicate that `kraft pkg pull` needs to occur
		return fmt.Errorf("cannot build without Unikraft core component source")
	}

	bopts.mopts = append(bopts.mopts, []make.MakeOption{
		make.WithProgressFunc(bopts.onProgress),
	}...)

	if !bopts.noPrepare {
		if err := a.Prepare(
			ctx,
			tc,
			append(
				bopts.mopts,
				make.WithProgressFunc(nil),
			)...); err != nil {
			return err
		}
	}

	return a.Make(ctx, tc, bopts.mopts...)
}

// LibraryNames return names for all libraries in this Compose config
func (a *ApplicationConfig) LibraryNames() []string {
	var names []string
	for k := range a.libraries {
		names = append(names, k)
	}

	sort.Strings(names)

	return names
}

// TargetNames return names for all targets in this Compose config
func (a *ApplicationConfig) TargetNames() []string {
	var names []string
	for _, k := range a.targets {
		names = append(names, k.Name())
	}

	sort.Strings(names)

	return names
}

// TargetByName returns the `*target.TargetConfig` based on an input name
func (a *ApplicationConfig) TargetByName(name string) (*target.TargetConfig, error) {
	if len(name) == 0 {
		return nil, fmt.Errorf("no target name specified in lookup")
	}

	for _, k := range a.targets {
		if k.Name() == name {
			return &k, nil
		}
	}

	return nil, fmt.Errorf("unknown target: %s", name)
}

// Components returns a unique list of Unikraft components which this
// applicatiton consists of
func (ac *ApplicationConfig) Components() ([]component.Component, error) {
	components := []component.Component{
		ac.unikraft,
	}

	// Change to error and correctly check if structure is uninitialized
	if ac.template.Source() != "" && !ac.template.IsUnpackedInProject() {
		return nil, fmt.Errorf("template source is not unpacked in project")
	}

	if ac.template.Name() != "" {
		components = append(components, ac.template)
	}

	for _, library := range ac.libraries {
		components = append(components, library)
	}

	// TODO: Get unique components from each target.  A target will contain at
	// least two components: the architecture and the platform.  Both of these
	// components can stem from the Unikraft core (in the case of built-in
	// architectures and components).
	// for _, targ := range ac.Targets {
	// 	components = append(components, targ)
	// }

	return components, nil
}

func (ac ApplicationConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

func (ac ApplicationConfig) PrintInfo() string {
	tree := treeprint.NewWithRoot(component.NameAndVersion(ac))

	tree.AddBranch(component.NameAndVersion(ac.unikraft))

	if len(ac.libraries) > 0 {
		libraries := tree.AddBranch(fmt.Sprintf("libraries (%d)", len(ac.libraries)))
		for _, library := range ac.libraries {
			libraries.AddNode(component.NameAndVersion(library))
		}
	}

	if len(ac.targets) > 0 {
		targets := tree.AddBranch(fmt.Sprintf("targets (%d)", len(ac.targets)))
		for _, target := range ac.targets {
			targ := targets.AddBranch(component.NameAndVersion(target))
			targ.AddNode(fmt.Sprintf("architecture: %s", component.NameAndVersion(target.Architecture)))
			targ.AddNode(fmt.Sprintf("platform:     %s", component.NameAndVersion(target.Platform)))
		}
	}

	return tree.String()
}

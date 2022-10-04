// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
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
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xlab/treeprint"

	"kraftkit.sh/exec"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/make"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/target"
)

const DefaultKConfigFile = ".config"

type Application interface {
	component.Component
}

type ApplicationConfig struct {
	component.ComponentConfig

	WorkingDir    string                `yaml:"-" json:"-"`
	Filename      string                `yaml:"-" json:"-"`
	OutDir        string                `yaml:",omitempty" json:"outdir,omitempty"`
	Unikraft      core.UnikraftConfig   `yaml:",omitempty" json:"unikraft,omitempty"`
	Libraries     lib.Libraries         `yaml:",omitempty" json:"libraries,omitempty"`
	Targets       target.Targets        `yaml:",omitempty" json:"targets,omitempty"`
	Extensions    component.Extensions  `yaml:",inline" json:"-"` // https://github.com/golang/go/issues/6213
	KraftFiles    []string              `yaml:"-" json:"-"`
	Configuration kconfig.KConfigValues `yaml:"-" json:"-"`
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

func (ac ApplicationConfig) KConfigMenu() (*kconfig.KConfigFile, error) {
	config_uk := filepath.Join(ac.WorkingDir, unikraft.Config_uk)
	if _, err := os.Stat(config_uk); err != nil {
		return nil, fmt.Errorf("could not read component Config.uk: %v", err)
	}

	return kconfig.Parse(config_uk)
}

func (ac ApplicationConfig) KConfigValues() (kconfig.KConfigValues, error) {
	vAll := kconfig.KConfigValues{}

	vCore, err := ac.Unikraft.KConfigValues()
	if err != nil {
		return nil, fmt.Errorf("could not read Unikraft core KConfig values: %v", err)
	}

	vAll.OverrideBy(vCore)

	for _, library := range ac.Libraries {
		vLib, err := library.KConfigValues()
		if err != nil {
			return nil, fmt.Errorf("could not %s's KConfig values: %v", library.Name(), err)
		}

		vAll.OverrideBy(vLib)
	}

	return vAll, nil
}

// KConfigFile returns the path to the application's .config file
func (ac *ApplicationConfig) KConfigFile() (string, error) {
	return filepath.Join(ac.WorkingDir, DefaultKConfigFile), nil
}

// IsConfigured returns a boolean to indicate whether the application has been
// previously configured.  This is deteremined by finding a non-empty `.config`
// file within the application's source directory
func (a *ApplicationConfig) IsConfigured() bool {
	k, err := a.KConfigFile()
	if err != nil {
		return false
	}

	f, err := os.Stat(k)
	return err == nil && !f.IsDir() && f.Size() > 0
}

// MakeArgs returns the populated `core.MakeArgs` based on the contents of the
// instantiated `ApplicationConfig`.  This information can be passed directly to
// Unikraft's build system.
func (a *ApplicationConfig) MakeArgs() (*core.MakeArgs, error) {
	var libraries []string

	for _, library := range a.Libraries {
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

	return &core.MakeArgs{
		OutputDir:      a.OutDir,
		ApplicationDir: a.WorkingDir,
		LibraryDirs:    strings.Join(libraries, core.MakeDelimeter),
	}, nil
}

// Make is a method which invokes Unikraft's build system.  You can pass in make
// options based on the `make` package.  Ultimately, this is an abstract method
// which will be used by a number of well-known make command goals by Unikraft's
// build system.
func (a *ApplicationConfig) Make(mopts ...make.MakeOption) error {
	coreSrc, err := a.Unikraft.SourceDir()
	if err != nil {
		return err
	}

	mopts = append(mopts,
		make.WithDirectory(coreSrc),
	)

	args, err := a.MakeArgs()
	if err != nil {
		return err
	}

	m, err := make.NewFromInterface(*args, mopts...)
	if err != nil {
		return err
	}

	// Unikraft currently requires each application to have a `Makefile.uk`
	// located within the working directory.  Create it if it does not exist:
	makefile_uk := filepath.Join(a.WorkingDir, unikraft.Makefile_uk)
	if _, err := os.Stat(makefile_uk); err != nil && os.IsNotExist(err) {
		if _, err := os.OpenFile(makefile_uk, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666); err != nil {
			return fmt.Errorf("could not create application %s: %v", makefile_uk, err)
		}
	}

	return m.Execute()
}

// SyncConfig updates the configuration
func (a *ApplicationConfig) SyncConfig(mopts ...make.MakeOption) error {
	return a.Make(append(mopts,
		make.WithExecOptions(
			exec.WithStdout(a.Log().Output()),
		),
		make.WithTarget("syncconfig"),
	)...)
}

// Defconfig updates the configuration
func (ac *ApplicationConfig) DefConfig(tc *target.TargetConfig, extra *kconfig.KConfigValues, mopts ...make.MakeOption) error {
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

	return ac.Make(append(mopts,
		make.WithExecOptions(
			exec.WithStdout(ac.Log().Output()),
		),
		make.WithTarget("defconfig"),
		make.WithVar("UK_DEFCONFIG", tmpfile.Name()),
	)...)
}

// Configure the application
func (a *ApplicationConfig) Configure(mopts ...make.MakeOption) error {
	return a.Make(append(mopts,
		make.WithExecOptions(
			exec.WithStdout(a.Log().Output()),
		),
		make.WithTarget("configure"),
	)...)
}

// Prepare the application
func (a *ApplicationConfig) Prepare(mopts ...make.MakeOption) error {
	return a.Make(append(mopts,
		make.WithExecOptions(
			exec.WithStdout(a.Log().Output()),
		),
		make.WithTarget("prepare"),
	)...)
}

// Clean the application
func (a *ApplicationConfig) Clean(mopts ...make.MakeOption) error {
	return a.Make(append(mopts,
		make.WithExecOptions(
			exec.WithStdout(a.Log().Output()),
		),
		make.WithTarget("clean"),
	)...)
}

// Delete the build folder of the application
func (a *ApplicationConfig) Properclean(mopts ...make.MakeOption) error {
	return a.Make(append(mopts,
		make.WithExecOptions(
			exec.WithStdout(a.Log().Output()),
		),
		make.WithTarget("properclean"),
	)...)
}

// Fetch component sources for the applications
func (a *ApplicationConfig) Fetch(mopts ...make.MakeOption) error {
	return a.Make(append(mopts,
		make.WithExecOptions(
			exec.WithStdout(a.Log().Output()),
		),
		make.WithTarget("fetch"),
	)...)
}

func (a *ApplicationConfig) Set(mopts ...make.MakeOption) error {
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

func (a *ApplicationConfig) Unset(mopts ...make.MakeOption) error {
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
func (a *ApplicationConfig) Build(opts ...BuildOption) error {
	bopts := &BuildOptions{}
	for _, o := range opts {
		err := o(bopts)
		if err != nil {
			return fmt.Errorf("could not apply build option: %v", err)
		}
	}

	if !a.Unikraft.IsUnpackedInProject() {
		// TODO: Produce better error messages (see #34).  In this case, we should
		// indicate that `kraft pkg pull` needs to occur
		return fmt.Errorf("cannot build without Unikraft core component source")
	}

	eopts := []exec.ExecOption{}
	if bopts.log != nil {
		eopts = append(eopts, exec.WithStdout(bopts.log.Output()))
	}

	bopts.mopts = append(bopts.mopts, []make.MakeOption{
		make.WithProgressFunc(bopts.onProgress),
		make.WithExecOptions(eopts...),
	}...)

	if !bopts.noSyncConfig {
		if err := a.SyncConfig(append(
			bopts.mopts,
			make.WithProgressFunc(nil),
		)...); err != nil {
			return err
		}
	}

	if !bopts.noFetch {
		if err := a.Fetch(append(
			bopts.mopts,
			make.WithProgressFunc(nil),
		)...); err != nil {
			return err
		}
	}

	return a.Make(bopts.mopts...)
}

// LibraryNames return names for all libraries in this Compose config
func (a *ApplicationConfig) LibraryNames() []string {
	var names []string
	for k := range a.Libraries {
		names = append(names, k)
	}

	sort.Strings(names)

	return names
}

// TargetNames return names for all targets in this Compose config
func (a *ApplicationConfig) TargetNames() []string {
	var names []string
	for _, k := range a.Targets {
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

	for _, k := range a.Targets {
		if k.Name() == name {
			return &k, nil
		}
	}

	return nil, fmt.Errorf("unknown target: %s", name)
}

// Components returns a unique list of Unikraft components which this
// applicatiton consists of
func (ac *ApplicationConfig) Components() []component.Component {
	components := []component.Component{
		ac.Unikraft,
	}

	for _, library := range ac.Libraries {
		components = append(components, library)
	}

	// TODO: Get unique components from each target.  A target will contain at
	// least two components: the architecture and the platform.  Both of these
	// components can stem from the Unikraft core (in the case of built-in
	// architectures and components).
	// for _, targ := range ac.Targets {
	// 	components = append(components, targ)
	// }

	return components
}

func (ac ApplicationConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

func (ac ApplicationConfig) PrintInfo(io *iostreams.IOStreams) error {
	tree := treeprint.NewWithRoot(component.NameAndVersion(ac))

	tree.AddBranch(component.NameAndVersion(ac.Unikraft))

	if len(ac.Libraries) > 0 {
		libraries := tree.AddBranch(fmt.Sprintf("libraries (%d)", len(ac.Libraries)))
		for _, library := range ac.Libraries {
			libraries.AddNode(component.NameAndVersion(library))
		}
	}

	if len(ac.Targets) > 0 {
		targets := tree.AddBranch(fmt.Sprintf("targets (%d)", len(ac.Targets)))
		for _, target := range ac.Targets {
			targ := targets.AddBranch(component.NameAndVersion(target))
			targ.AddNode(fmt.Sprintf("architecture: %s", component.NameAndVersion(target.Architecture)))
			targ.AddNode(fmt.Sprintf("platform:     %s", component.NameAndVersion(target.Platform)))
		}
	}

	fmt.Fprintln(io.Out, tree.String())

	return nil
}

// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2020 The Compose Specification Authors.
// Copyright 2022 Unikraft GmbH. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/dotenv"
	"github.com/pkg/errors"

	"kraftkit.sh/internal/errs"
	"kraftkit.sh/kconfig"

	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/config"
)

// ProjectOptions groups the command line options recommended for a Compose implementation
type ProjectOptions struct {
	Name          string
	WorkingDir    string
	ConfigPaths   []string
	Configuration kconfig.KConfigValues
	DotConfigFile string
	log           log.Logger
	loadOptions   []func(*LoaderOptions)
}

type ProjectOptionsFn func(*ProjectOptions) error

// NewProjectOptions creates ProjectOptions
func NewProjectOptions(configs []string, opts ...ProjectOptionsFn) (*ProjectOptions, error) {
	options := &ProjectOptions{
		ConfigPaths:   configs,
		Configuration: kconfig.KConfigValues{},
	}
	for _, o := range opts {
		err := o(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

// WithLogger defines the log.Logger
func WithLogger(l log.Logger) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.log = l
		return nil
	}
}

// WithName defines ProjectOptions' name
func WithName(name string) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.Name = name
		return nil
	}
}

// WithWorkingDirectory defines ProjectOptions' working directory
func WithWorkingDirectory(wd string) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		if wd == "" {
			return nil
		}
		abs, err := filepath.Abs(wd)
		if err != nil {
			return err
		}
		o.WorkingDir = abs
		return nil
	}
}

// WithConfig defines a key=value set of variables used for kraft file
// interpolation as well as with Unikraft's build system
func WithConfig(config []string) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		for k, v := range getAsEqualsMap(config) {
			o.Configuration.Set(k, v)
		}
		return nil
	}
}

// WithConfigFile set an alternate config file
func WithConfigFile(file string) ProjectOptionsFn {
	return func(options *ProjectOptions) error {
		options.DotConfigFile = file
		return nil
	}
}

func withDotConfig(o *ProjectOptions) error {
	if o.DotConfigFile == "" {
		wd, err := o.GetWorkingDir()
		if err != nil {
			return err
		}

		o.DotConfigFile = filepath.Join(wd, kconfig.DotConfigFileName)
	}

	dotConfigFile := o.DotConfigFile

	abs, err := filepath.Abs(dotConfigFile)
	if err != nil {
		return err
	}

	dotConfigFile = abs

	s, err := os.Stat(dotConfigFile)
	if os.IsNotExist(err) {
		if o.DotConfigFile != "" {
			return errors.Errorf("couldn't find config file: %s", o.DotConfigFile)
		}
		return nil
	}

	if err != nil {
		return err
	}

	if s.IsDir() {
		if o.DotConfigFile == "" {
			return nil
		}
		return errors.Errorf("%s is a directory", dotConfigFile)
	}

	file, err := os.Open(dotConfigFile)
	if err != nil {
		return err
	}

	defer file.Close()

	config := kconfig.KConfigValues{}

	notInConfigSet := make(map[string]interface{})
	env, err := dotenv.ParseWithLookup(file, func(k string) (string, bool) {
		v, ok := os.LookupEnv(k)
		if !ok {
			config.Unset(k)
			return "", true
		}

		return v, true
	})
	if err != nil {
		return err
	}

	for k, v := range env {
		if _, ok := notInConfigSet[k]; ok {
			continue
		}

		config.Set(k, v)
	}

	o.Configuration = config

	return nil
}

// WithDotConfig imports configuration variables from .config file
func WithDotConfig(enforce bool) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		if err := withDotConfig(o); err != nil && enforce {
			return err
		}

		return nil
	}
}

// WithInterpolation set ProjectOptions to enable/skip interpolation
func WithInterpolation(interpolation bool) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.loadOptions = append(o.loadOptions, func(options *LoaderOptions) {
			options.SkipInterpolation = !interpolation
		})
		return nil
	}
}

// WithNormalization set ProjectOptions to enable/skip normalization
func WithNormalization(normalization bool) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.loadOptions = append(o.loadOptions, func(options *LoaderOptions) {
			options.SkipNormalization = !normalization
		})
		return nil
	}
}

// WithResolvedPaths set ProjectOptions to enable paths resolution
func WithResolvedPaths(resolve bool) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.loadOptions = append(o.loadOptions, func(options *LoaderOptions) {
			options.ResolvePaths = resolve
		})
		return nil
	}
}

// DefaultFileNames defines the kraft file names for auto-discovery (in order
// of preference)
var DefaultFileNames = []string{
	"kraft.yaml",
	"kraft.yml",
	"Kraftfile.yml",
	"Kraftfile.yaml",
	"Kraftfile",
}

// WithDefaultConfigPath searches for default config files from working
// directory
func WithDefaultConfigPath() ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		if len(o.ConfigPaths) > 0 {
			return nil
		}

		pwd, err := o.GetWorkingDir()
		if err != nil {
			return err
		}

		for {
			candidates := findFiles(DefaultFileNames, pwd)
			if len(candidates) > 0 {
				winner := candidates[0]
				if len(candidates) > 1 {
					o.log.Warn("Found multiple config files with supported names: %s", strings.Join(candidates, ", "))
					o.log.Warn("Using %s", winner)
				}

				o.ConfigPaths = append(o.ConfigPaths, winner)

				return nil
			}

			parent := filepath.Dir(pwd)
			if parent == pwd {
				// no config file found, but that's not a blocker if caller only needs project name
				return nil
			}

			pwd = parent
		}
	}
}

// WithPackageManager provides access to the package manager of choice to be
// able to retrieve component sources
func WithPackageManager(pm *packmanager.PackageManager) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.loadOptions = append(o.loadOptions, func(options *LoaderOptions) {
			options.PackageManager = pm
		})
		return nil
	}
}

func findFiles(names []string, pwd string) []string {
	candidates := []string{}
	for _, n := range names {
		f := filepath.Join(pwd, n)
		if _, err := os.Stat(f); err == nil {
			candidates = append(candidates, f)
		}
	}
	return candidates
}

// IsWorkdirInitialized provides a quick check to determine if whether one of
// the supported project files (Kraftfiles) is present within a provided working
// directory.
func IsWorkdirInitialized(dir string) bool {
	return len(findFiles(DefaultFileNames, dir)) > 0
}

func (o ProjectOptions) GetWorkingDir() (string, error) {
	if o.WorkingDir != "" {
		return o.WorkingDir, nil
	}
	for _, path := range o.ConfigPaths {
		if path != "-" {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return "", err
			}
			return filepath.Dir(absPath), nil
		}
	}
	return os.Getwd()
}

// NewApplicationFromOptions load a kraft project based on command line options
func NewApplicationFromOptions(popts *ProjectOptions, copts ...component.ComponentOption) (*ApplicationConfig, error) {
	configPaths, err := getConfigPathsFromOptions(popts)
	if err != nil {
		return nil, err
	}

	var configs []config.ConfigFile
	for _, f := range configPaths {
		var b []byte
		if f == "-" {
			b, err = ioutil.ReadAll(os.Stdin)
			if err != nil {
				return nil, err
			}
		} else {
			f, err := filepath.Abs(f)
			if err != nil {
				return nil, err
			}
			b, err = ioutil.ReadFile(f)
			if err != nil {
				return nil, err
			}
		}
		configs = append(configs, config.ConfigFile{
			Filename: f,
			Content:  b,
		})
	}

	workingDir, err := popts.GetWorkingDir()
	if err != nil {
		return nil, err
	}
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, err
	}

	popts.loadOptions = append(popts.loadOptions,
		withNamePrecedence(absWorkingDir, popts),
	)

	popts.loadOptions = append(popts.loadOptions,
		withComponentOptions(copts...),
	)

	if popts.log != nil {
		popts.loadOptions = append(popts.loadOptions,
			withLoaderLogger(popts.log),
		)
	}

	project, err := Load(config.ConfigDetails{
		ConfigFiles:   configs,
		WorkingDir:    workingDir,
		Configuration: popts.Configuration,
	}, popts.loadOptions...)
	if err != nil {
		return nil, err
	}

	WithKraftFiles(configPaths)(project)
	return project, nil
}

// getConfigPathsFromOptions retrieves the config files for project based on project options
func getConfigPathsFromOptions(options *ProjectOptions) ([]string, error) {
	if len(options.ConfigPaths) != 0 {
		return absolutePaths(options.ConfigPaths)
	}

	return nil, errors.Wrap(errs.ErrNotFound, "no configuration file provided")
}

func absolutePaths(p []string) ([]string, error) {
	var paths []string
	for _, f := range p {
		if f == "-" {
			paths = append(paths, f)
			continue
		}
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		f = abs
		if _, err := os.Stat(f); err != nil {
			return nil, err
		}
		paths = append(paths, f)
	}
	return paths, nil
}

// getAsEqualsMap split key=value formatted strings into a key : value map
func getAsEqualsMap(em []string) map[string]string {
	m := make(map[string]string)
	for _, v := range em {
		kv := strings.SplitN(v, "=", 2)
		m[kv[0]] = kv[1]
	}

	return m
}

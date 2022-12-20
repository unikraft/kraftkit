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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/dotenv"
	"github.com/pkg/errors"

	"kraftkit.sh/kconfig"
)

// ProjectOptions groups the command line options recommended for a Compose
// implementation
type ProjectOptions struct {
	Name          string
	WorkingDir    string
	ConfigPaths   []string
	Configuration kconfig.KConfigValues
	DotConfigFile string
	loadOptions   []func(*LoaderOptions)
}

func (popts *ProjectOptions) GetWorkingDir() (string, error) {
	if popts.WorkingDir != "" {
		return popts.WorkingDir, nil
	}

	for _, path := range popts.ConfigPaths {
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

type ProjectOption func(*ProjectOptions) error

// NewProjectOptions creates ProjectOptions
func NewProjectOptions(configs []string, opts ...ProjectOption) (*ProjectOptions, error) {
	popts := &ProjectOptions{
		ConfigPaths:   configs,
		Configuration: kconfig.KConfigValues{},
	}

	for _, o := range opts {
		err := o(popts)
		if err != nil {
			return nil, err
		}
	}

	return popts, nil
}

// WithProjectName defines ProjectOptions' name
func WithProjectName(name string) ProjectOption {
	return func(popts *ProjectOptions) error {
		popts.Name = name
		return nil
	}
}

// WithProjectWorkdir defines ProjectOptions' working directory
func WithProjectWorkdir(pwd string) ProjectOption {
	return func(popts *ProjectOptions) error {
		if pwd == "" {
			return nil
		}

		abs, err := filepath.Abs(pwd)
		if err != nil {
			return err
		}

		popts.WorkingDir = abs

		return nil
	}
}

// WithProjectConfig defines a key=value set of variables used for kraft file
// interpolation as well as with Unikraft's build system
func WithProjectConfig(config []string) ProjectOption {
	return func(popts *ProjectOptions) error {
		for k, v := range getAsEqualsMap(config) {
			popts.Configuration.Set(k, v)
		}
		return nil
	}
}

// WithProjectConfigFile set an alternate config file
func WithProjectConfigFile(file string) ProjectOption {
	return func(popts *ProjectOptions) error {
		popts.DotConfigFile = file
		return nil
	}
}

func withProjectDotConfig(popts *ProjectOptions) error {
	if popts.DotConfigFile == "" {
		wd, err := popts.GetWorkingDir()
		if err != nil {
			return err
		}

		popts.DotConfigFile = filepath.Join(wd, kconfig.DotConfigFileName)
	}

	dotConfigFile := popts.DotConfigFile

	abs, err := filepath.Abs(dotConfigFile)
	if err != nil {
		return err
	}

	dotConfigFile = abs

	s, err := os.Stat(dotConfigFile)
	if os.IsNotExist(err) {
		if popts.DotConfigFile != "" {
			return errors.Errorf("couldn't find config file: %s", popts.DotConfigFile)
		}
		return nil
	}

	if err != nil {
		return err
	}

	if s.IsDir() {
		if popts.DotConfigFile == "" {
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

	popts.Configuration = config

	return nil
}

// WithProjectDotConfig imports configuration variables from .config file
func WithProjectDotConfig(enforce bool) ProjectOption {
	return func(popts *ProjectOptions) error {
		if err := withProjectDotConfig(popts); err != nil && enforce {
			return err
		}

		return nil
	}
}

// WithProjectInterpolation set ProjectOptions to enable/skip interpolation
func WithProjectInterpolation(interpolation bool) ProjectOption {
	return func(popts *ProjectOptions) error {
		popts.loadOptions = append(popts.loadOptions, func(options *LoaderOptions) {
			options.SkipInterpolation = !interpolation
		})
		return nil
	}
}

// WithProjectNormalization set ProjectOptions to enable/skip normalization
func WithProjectNormalization(normalization bool) ProjectOption {
	return func(popts *ProjectOptions) error {
		popts.loadOptions = append(popts.loadOptions, func(options *LoaderOptions) {
			options.SkipNormalization = !normalization
		})
		return nil
	}
}

// WithProjectResolvedPaths set ProjectOptions to enable paths resolution
func WithProjectResolvedPaths(resolve bool) ProjectOption {
	return func(popts *ProjectOptions) error {
		popts.loadOptions = append(popts.loadOptions, func(options *LoaderOptions) {
			options.ResolvePaths = resolve
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

// getAsEqualsMap split key=value formatted strings into a key : value map
func getAsEqualsMap(em []string) map[string]string {
	m := make(map[string]string)
	for _, v := range em {
		kv := strings.SplitN(v, "=", 2)
		m[kv[0]] = kv[1]
	}

	return m
}

// WithProjectDefaultConfigPath searches for default config files from working
// directory
func WithProjectDefaultConfigPath() ProjectOption {
	return func(popts *ProjectOptions) error {
		if len(popts.ConfigPaths) > 0 {
			return nil
		}

		pwd, err := popts.GetWorkingDir()
		if err != nil {
			return err
		}

		for {
			candidates := findFiles(DefaultFileNames, pwd)
			if len(candidates) > 0 {
				if len(candidates) > 1 {
					return fmt.Errorf("found multiple config files with supported names: %s", strings.Join(candidates, ", "))
				}

				popts.ConfigPaths = append(popts.ConfigPaths, candidates[0])

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

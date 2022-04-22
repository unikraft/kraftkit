// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2020 The Compose Specification Authors.
// Copyright 2022 Unikraft UG. All rights reserved.
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

package schema

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/dotenv"
	"github.com/pkg/errors"
	"go.unikraft.io/kit/internal/errs"
	"go.unikraft.io/kit/pkg/unikraft/app"
	"go.unikraft.io/kit/pkg/unikraft/config"
)

// ProjectOptions groups the command line options recommended for a Compose implementation
type ProjectOptions struct {
	Name        string
	WorkingDir  string
	ConfigPaths []string
	Environment map[string]string
	EnvFile     string
	loadOptions []func(*LoaderOptions)
}

type ProjectOptionsFn func(*ProjectOptions) error

// NewProjectOptions creates ProjectOptions
func NewProjectOptions(configs []string, opts ...ProjectOptionsFn) (*ProjectOptions, error) {
	options := &ProjectOptions{
		ConfigPaths: configs,
		Environment: map[string]string{},
	}
	for _, o := range opts {
		err := o(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
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

// WithEnv defines a key=value set of variables used for compose file interpolation
func WithEnv(env []string) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		for k, v := range getAsEqualsMap(env) {
			o.Environment[k] = v
		}
		return nil
	}
}

// WithEnvFile set an alternate env file
func WithEnvFile(file string) ProjectOptionsFn {
	return func(options *ProjectOptions) error {
		options.EnvFile = file
		return nil
	}
}

// WithDotEnv imports environment variables from .env file
func WithDotEnv(o *ProjectOptions) error {
	dotEnvFile := o.EnvFile
	if dotEnvFile == "" {
		wd, err := o.GetWorkingDir()
		if err != nil {
			return err
		}

		dotEnvFile = filepath.Join(wd, ".env")
	}

	abs, err := filepath.Abs(dotEnvFile)
	if err != nil {
		return err
	}

	dotEnvFile = abs

	s, err := os.Stat(dotEnvFile)
	if os.IsNotExist(err) {
		if o.EnvFile != "" {
			return errors.Errorf("Couldn't find env file: %s", o.EnvFile)
		}
		return nil
	}

	if err != nil {
		return err
	}

	if s.IsDir() {
		if o.EnvFile == "" {
			return nil
		}
		return errors.Errorf("%s is a directory", dotEnvFile)
	}

	file, err := os.Open(dotEnvFile)
	if err != nil {
		return err
	}

	defer file.Close()

	notInEnvSet := make(map[string]interface{})
	env, err := dotenv.ParseWithLookup(file, func(k string) (string, bool) {
		v, ok := os.LookupEnv(k)
		if !ok {
			notInEnvSet[k] = nil
			return "", true
		}

		return v, true
	})
	if err != nil {
		return err
	}

	for k, v := range env {
		if _, ok := notInEnvSet[k]; ok {
			continue
		}
		o.Environment[k] = v
	}

	return nil
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

// ProjectFromOptions load a kraft project based on command line options
func ProjectFromOptions(options *ProjectOptions) (*app.ApplicationConfig, error) {
	configPaths, err := getConfigPathsFromOptions(options)
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

	workingDir, err := options.GetWorkingDir()
	if err != nil {
		return nil, err
	}
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, err
	}

	options.loadOptions = append(options.loadOptions,
		withNamePrecedenceLoad(absWorkingDir, options),
	)

	project, err := Load(config.ConfigDetails{
		ConfigFiles: configs,
		WorkingDir:  workingDir,
		Environment: options.Environment,
	}, options.loadOptions...)
	if err != nil {
		return nil, err
	}

	project.KraftFiles = configPaths
	return project, nil
}

// getConfigPathsFromOptions retrieves the config files for project based on project options
func getConfigPathsFromOptions(options *ProjectOptions) ([]string, error) {
	if len(options.ConfigPaths) != 0 {
		return absolutePaths(options.ConfigPaths)
	}

	return nil, errors.Wrap(errs.ErrNotFound, "no configuration file provided")
}

func withNamePrecedenceLoad(absWorkingDir string, options *ProjectOptions) func(*LoaderOptions) {
	return func(opts *LoaderOptions) {
		if options.Name != "" {
			opts.SetProjectName(options.Name, true)
		} else {
			opts.SetProjectName(filepath.Base(absWorkingDir), false)
		}
	}
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

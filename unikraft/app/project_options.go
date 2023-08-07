// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2020 The Compose Specification Authors.
// Copyright 2022 Unikraft GmbH. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	interp "github.com/compose-spec/compose-go/interpolation"
	"github.com/compose-spec/compose-go/template"

	"kraftkit.sh/kconfig"
)

// kraftfile is a filename and the contents of the file
type Kraftfile struct {
	// path is the path of the yaml configuration file
	path string

	// content is the raw yaml content. Will be loaded from Filename if not set
	content []byte

	// config if the yaml tree for this config file. Will be parsed from Content
	// if not set
	config map[string]interface{}
}

// ProjectOptions group configuration options used to instantiate a new
// ApplicationConfig from a working directory and a kraftfile
type ProjectOptions struct {
	name              string
	workdir           string
	kraftfile         *Kraftfile
	kconfig           kconfig.KeyValueMap
	skipValidation    bool
	skipInterpolation bool
	skipNormalization bool
	resolvePaths      bool
	interpolate       *interp.Options

	// Indicates when the projectName was imperatively set or guessed from path
	projectNameImperativelySet bool
}

// Workdir returns the working directory determined by provided kraft files
func (popts *ProjectOptions) Workdir() (string, error) {
	if popts.workdir != "" {
		return popts.workdir, nil
	}

	if popts.kraftfile == nil {
		return os.Getwd()
	}

	if popts.kraftfile.path != "" && popts.kraftfile.path != "-" {
		absPath, err := filepath.Abs(popts.kraftfile.path)
		if err != nil {
			return "", err
		}

		return filepath.Dir(absPath), nil
	}

	return os.Getwd()
}

// RelativePath resolve a relative path based project's working directory
func (popts *ProjectOptions) RelativePath(path string) string {
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(popts.workdir, path)
}

// LookupConfig provides a lookup function for config variables
func (popts *ProjectOptions) LookupConfig(key string) (string, bool) {
	v, ok := popts.kconfig[key]
	return v.Value, ok
}

// SetProjectName sets the project name along with whether the name is set
// imperatively (i.e. whether it was derived from the working directory)
func (popts *ProjectOptions) SetProjectName(name string, imperativelySet bool) {
	popts.name = name
	popts.projectNameImperativelySet = imperativelySet
}

// GetProjectName returns the project name and whether the project name was set
// impartively (i.e. by the working directory's name)
func (popts *ProjectOptions) GetProjectName() (string, bool) {
	return popts.name, popts.projectNameImperativelySet
}

// AddKraftfile adds and extracts the file contents and attaches it to the
// ProjectOptions.
func (popts *ProjectOptions) AddKraftfile(file string) error {
	var b []byte
	var err error

	if file == "-" {
		b, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}

	} else {
		abs, err := filepath.Abs(file)
		if err != nil {
			return err
		}

		f := abs
		if _, err := os.Stat(f); err != nil {
			return err
		}

		b, err = os.ReadFile(f)
		if err != nil {
			return err
		}
	}

	popts.kraftfile = &Kraftfile{
		path:    file,
		content: b,
	}

	return nil
}

// AddKraftfileFromBytes adds and extracts the file contents from a given byte slice
// and attaches it to the ProjectOptions.
func (popts *ProjectOptions) AddKraftfileFromBytes(content []byte) error {
	popts.kraftfile = &Kraftfile{
		content: content,
	}

	return nil
}

type ProjectOption func(*ProjectOptions) error

// NewProjectOptions creates ProjectOptions
func NewProjectOptions(opts ...ProjectOption) (*ProjectOptions, error) {
	popts := &ProjectOptions{
		kconfig: kconfig.KeyValueMap{},
	}

	popts.interpolate = &interp.Options{
		Substitute:  template.Substitute,
		LookupValue: popts.LookupConfig,
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
		popts.name = name
		return nil
	}
}

// WithProjectWorkdir defines ProjectOptions' working directory
func WithProjectWorkdir(workdir string) ProjectOption {
	return func(popts *ProjectOptions) error {
		if workdir == "" {
			return nil
		}

		abs, err := filepath.Abs(workdir)
		if err != nil {
			return err
		}

		popts.workdir = abs

		return nil
	}
}

// WithProjectConfig defines a key=value set of variables used for kraft file
// interpolation as well as with Unikraft's build system
func WithProjectConfig(config []string) ProjectOption {
	return func(popts *ProjectOptions) error {
		for k, v := range getAsEqualsMap(config) {
			popts.kconfig.Set(k, v)
		}
		return nil
	}
}

// WithProjectKraftfile adds a kraft file to the project
func WithProjectKraftfile(file string) ProjectOption {
	return func(popts *ProjectOptions) error {
		return popts.AddKraftfile(file)
	}
}

// WithProjectKraftfile adds a kraft file to the project
func WithProjectKraftfileFromBytes(content []byte) ProjectOption {
	return func(popts *ProjectOptions) error {
		return popts.AddKraftfileFromBytes(content)
	}
}

// WithProjectInterpolation set ProjectOptions to enable/skip interpolation
func WithProjectInterpolation(interpolation bool) ProjectOption {
	return func(popts *ProjectOptions) error {
		popts.skipInterpolation = !interpolation
		return nil
	}
}

// WithProjectNormalization set ProjectOptions to enable/skip normalization
func WithProjectNormalization(normalization bool) ProjectOption {
	return func(popts *ProjectOptions) error {
		popts.skipNormalization = !normalization
		return nil
	}
}

// WithProjectResolvedPaths set ProjectOptions to enable paths resolution
func WithProjectResolvedPaths(resolve bool) ProjectOption {
	return func(popts *ProjectOptions) error {
		popts.resolvePaths = resolve
		return nil
	}
}

// WithSkipValidation sets the LoaderOptions to skip validation when loading
// sections
func WithProjectSkipValidation(skipValidation bool) ProjectOption {
	return func(popts *ProjectOptions) error {
		popts.skipValidation = true
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

// WithProjectDefaultKraftfiles searches for default kraft files from working
// directory
func WithProjectDefaultKraftfiles() ProjectOption {
	return func(popts *ProjectOptions) error {
		if popts.kraftfile != nil {
			return nil
		}

		pwd, err := popts.Workdir()
		if err != nil {
			return err
		}

		for {
			candidates := findFiles(DefaultFileNames, pwd)
			if len(candidates) > 0 {
				if len(candidates) > 1 {
					return fmt.Errorf("found multiple config files with supported names: %s", strings.Join(candidates, ", "))
				}

				return popts.AddKraftfile(candidates[0])
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

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

	"github.com/pkg/errors"

	"kraftkit.sh/internal/errs"

	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/config"
)

// DefaultFileNames defines the kraft file names for auto-discovery (in order
// of preference)
var DefaultFileNames = []string{
	"kraft.yaml",
	"kraft.yml",
	"Kraftfile.yml",
	"Kraftfile.yaml",
	"Kraftfile",
}

// IsWorkdirInitialized provides a quick check to determine if whether one of
// the supported project files (Kraftfiles) is present within a provided working
// directory.
func IsWorkdirInitialized(dir string) bool {
	return len(findFiles(DefaultFileNames, dir)) > 0
}

// NewProjectFromOptions load a kraft project based on command line options
func NewProjectFromOptions(popts *ProjectOptions, copts ...component.ComponentOption) (*ApplicationConfig, error) {
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

// getConfigPathsFromOptions retrieves the config files for project based on project options
func getConfigPathsFromOptions(options *ProjectOptions) ([]string, error) {
	if len(options.ConfigPaths) != 0 {
		return absolutePaths(options.ConfigPaths)
	}

	return nil, errors.Wrap(errs.ErrNotFound, "no configuration file provided")
}

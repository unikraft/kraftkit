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
	"path/filepath"

	interp "github.com/compose-spec/compose-go/interpolation"

	"kraftkit.sh/unikraft/component"
)

// LoaderOptions supported by Load
type LoaderOptions struct {
	// Skip schema validation
	SkipValidation bool
	// Skip interpolation
	SkipInterpolation bool
	// Skip normalization
	SkipNormalization bool
	// Resolve paths
	ResolvePaths bool
	// Interpolation options
	Interpolate *interp.Options
	// Set project projectName
	projectName string
	// Indicates when the projectName was imperatively set or guessed from path
	projectNameImperativelySet bool
	// Slice of component options to apply to each loaded component
	componentOptions []component.ComponentOption
}

func (o *LoaderOptions) SetProjectName(name string, imperativelySet bool) {
	o.projectName = normalizeProjectName(name)
	o.projectNameImperativelySet = imperativelySet
}

func (o LoaderOptions) GetProjectName() (string, bool) {
	return o.projectName, o.projectNameImperativelySet
}

// WithSkipValidation sets the LoaderOptions to skip validation when loading
// sections
func WithSkipValidation(opts *LoaderOptions) {
	opts.SkipValidation = true
}

func withNamePrecedence(absWorkingDir string, popts *ProjectOptions) func(*LoaderOptions) {
	return func(lopts *LoaderOptions) {
		if popts.Name != "" {
			lopts.SetProjectName(popts.Name, true)
		} else {
			lopts.SetProjectName(filepath.Base(absWorkingDir), false)
		}
	}
}

func withComponentOptions(copts ...component.ComponentOption) func(*LoaderOptions) {
	return func(lopts *LoaderOptions) {
		lopts.componentOptions = copts
	}
}

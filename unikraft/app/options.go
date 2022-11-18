// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Cezar Craciunoiu <cezar@unikraft.io>
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

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/target"
	"kraftkit.sh/unikraft/template"
)

// ApplicationOption is a function that operates on an ApplicationConfig
type ApplicationOption func(ao *ApplicationConfig) error

// NewApplicationOptions accepts a series of options and returns a rendered
// *ApplicationOptions structure
func NewApplicationOptions(aopts ...ApplicationOption) (*ApplicationConfig, error) {
	ao := &ApplicationConfig{}

	for _, o := range aopts {
		if err := o(ao); err != nil {
			return nil, fmt.Errorf("could not apply option: %v", err)
		}
	}

	return ao, nil
}

// WithWorkingDir sets the application's working directory
func WithWorkingDir(workingDir string) ApplicationOption {
	return func(ao *ApplicationConfig) error {
		ao.workingDir = workingDir
		return nil
	}
}

// WithFilename sets the application's file name
func WithFilename(filename string) ApplicationOption {
	return func(ao *ApplicationConfig) error {
		ao.filename = filename
		return nil
	}
}

// WithOutDir sets the application's output directory
func WithOutDir(outDir string) ApplicationOption {
	return func(ao *ApplicationConfig) error {
		ao.outDir = outDir
		return nil
	}
}

// WithTemplate sets the application's template
func WithTemplate(template template.TemplateConfig) ApplicationOption {
	return func(ao *ApplicationConfig) error {
		ao.template = template
		return nil
	}
}

// WithUnikraft sets the application's core
func WithUnikraft(unikraft core.UnikraftConfig) ApplicationOption {
	return func(ao *ApplicationConfig) error {
		ao.unikraft = unikraft
		return nil
	}
}

// WithLibraries sets the application's library list
func WithLibraries(libraries lib.Libraries) ApplicationOption {
	return func(ao *ApplicationConfig) error {
		ao.libraries = libraries
		return nil
	}
}

// WithTargets sets the application's target list
func WithTargets(targets target.Targets) ApplicationOption {
	return func(ao *ApplicationConfig) error {
		ao.targets = targets
		return nil
	}
}

// WithExtensions sets the application's extension list
func WithExtensions(extensions component.Extensions) ApplicationOption {
	return func(ao *ApplicationConfig) error {
		ao.extensions = extensions
		return nil
	}
}

// WithKraftFiles sets the application's kraft yaml files
func WithKraftFiles(kraftFiles []string) ApplicationOption {
	return func(ao *ApplicationConfig) error {
		ao.kraftFiles = kraftFiles
		return nil
	}
}

// WithConfiguration sets the application's kconfig list
func WithConfiguration(configuration kconfig.KConfigValues) ApplicationOption {
	return func(ao *ApplicationConfig) error {
		ao.configuration = configuration
		return nil
	}
}

// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft UG.  All rights reserved.
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
	"os"
	"path/filepath"
	"sort"

	"go.unikraft.io/kit/pkg/unikraft"
	"go.unikraft.io/kit/pkg/unikraft/component"
	"go.unikraft.io/kit/pkg/unikraft/core"
	"go.unikraft.io/kit/pkg/unikraft/lib"
	"go.unikraft.io/kit/pkg/unikraft/target"
)

type Application interface {
	component.Component
}

type ApplicationConfig struct {
	component.ComponentConfig

	WorkingDir    string               `yaml:"-" json:"-"`
	Filename      string               `yaml:"-" json:"-"`
	OutDir        string               `yaml:",omitempty" json:"outdir,omitempty"`
	Unikraft      core.UnikraftConfig  `yaml:",omitempty" json:"unikraft,omitempty"`
	Libraries     lib.Libraries        `yaml:",omitempty" json:"libraries,omitempty"`
	Targets       target.Targets       `yaml:",omitempty" json:"targets,omitempty"`
	Extensions    component.Extensions `yaml:",inline" json:"-"` // https://github.com/golang/go/issues/6213
	KraftFiles    []string             `yaml:"-" json:"-"`
	Configuration map[string]string    `yaml:"-" json:"-"`
}

func (ac ApplicationConfig) Name() string {
	return ac.ComponentConfig.Name
}

func (ac ApplicationConfig) Version() string {
	return ac.ComponentConfig.Version
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

func (ac *ApplicationConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

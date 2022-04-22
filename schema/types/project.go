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

package types

import (
	"os"
	"path/filepath"
	"sort"

	"go.unikraft.io/kit/pkg/unikraft/component"
	"go.unikraft.io/kit/pkg/unikraft/target"
)

type Project struct {
	Name        string               `yaml:"name,omitempty" json:"name,omitempty"`
	WorkingDir  string               `yaml:"-" json:"-"`
	Unikraft    UnikraftConfig       `yaml:",omitempty" json:"unikraft,omitempty"`
	Libraries   Libraries            `yaml:",omitempty" json:"libraries,omitempty"`
	Targets     target.Targets       `yaml:",omitempty" json:"targets,omitempty"`
	Extensions  component.Extensions `yaml:",inline" json:"-"` // https://github.com/golang/go/issues/6213
	KraftFiles  []string             `yaml:"-" json:"-"`
	Environment map[string]string    `yaml:"-" json:"-"`
}

// LibraryNames return names for all libraries in this Compose config
func (p Project) LibraryNames() []string {
	var names []string
	for k := range p.Libraries {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// TargetNames return names for all targets in this Compose config
func (p Project) TargetNames() []string {
	var names []string
	for _, k := range p.Targets {
		names = append(names, k.Name)
	}
	sort.Strings(names)
	return names
}

// RelativePath resolve a relative path based project's working directory
func (p *Project) RelativePath(path string) string {
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(p.WorkingDir, path)
}

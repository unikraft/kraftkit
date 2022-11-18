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

package config

import (
	"os"
	"path/filepath"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/target"
	"kraftkit.sh/unikraft/template"
)

// ConfigDetails are the details about a group of ConfigFiles
type ConfigDetails struct {
	Version       string
	WorkingDir    string
	ConfigFiles   []ConfigFile
	Configuration kconfig.KConfigValues
}

// LookupConfig provides a lookup function for config variables
func (cd ConfigDetails) LookupConfig(key string) (string, bool) {
	v, ok := cd.Configuration[key]
	return v.Value, ok
}

// RelativePath resolve a relative path based project's working directory
func (cd ConfigDetails) RelativePath(path string) string {
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(cd.WorkingDir, path)
}

// ConfigFile is a filename and the contents of the file as a Dict
type ConfigFile struct {
	// Filename is the name of the yaml configuration file
	Filename string

	// Content is the raw yaml content. Will be loaded from Filename if not set
	Content []byte

	// Config if the yaml tree for this config file. Will be parsed from Content
	// if not set
	Config map[string]interface{}
}

// Config is a full kraft file configuration and model
type Config struct {
	Filename  string                  `yaml:"-" json:"-"`
	Name      string                  `yaml:",omitempty" json:"name,omitempty"`
	OutDir    string                  `yaml:",omitempty" json:"outdir,omitempty"`
	Template  template.TemplateConfig `yaml:",omitempty" json:"template,omitempty"`
	Unikraft  core.UnikraftConfig     `yaml:",omitempty" json:"unikraft,omitempty"`
	Libraries lib.Libraries           `yaml:",omitempty" json:"libraries,omitempty"`
	Targets   target.Targets          `yaml:",omitempty" json:"targets,omitempty"`

	Extensions component.Extensions `yaml:",inline" json:"-"`
}

// ShellCommand is a string or list of string args
type ShellCommand []string

// StringList is a type for fields that can be a string or list of strings
type StringList []string

// StringOrNumberList is a type for fields that can be a list of strings or
// numbers
type StringOrNumberList []string

// MappingWithEquals is a mapping type that can be converted from a list of
// key[=value] strings. For the key with an empty value (`key=`), the mapped
// value is set to a pointer to `""`. For the key without value (`key`), the
// mapped value is set to nil.
type MappingWithEquals map[string]*string

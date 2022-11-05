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

package component

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
)

// ComponentConfig is the shared attribute structure provided to all
// microlibraries, whether they are a library, platform, architecture, an
// application itself or the Unikraft core.
type ComponentConfig struct {
	Name          string                `yaml:",omitempty" json:"-"`
	Version       string                `yaml:",omitempty" json:"version,omitempty"`
	Source        string                `yaml:",omitempty" json:"source,omitempty"`
	Configuration kconfig.KConfigValues `yaml:",omitempty" json:"kconfig,omitempty"`

	Extensions map[string]interface{} `yaml:",inline" json:"-"`

	workdir string
	pm      *packmanager.PackageManager
	log     log.Logger
	ctype   unikraft.ComponentType // embed the component type within the config

	// Context should contain all implementation-specific options, using
	// `context.WithValue`
	ctx context.Context
}

// Component is the abstract interface for managing the individual microlibrary
type Component interface {
	// Name returns the component name
	Name() string

	// Source returns the component source
	Source() string

	// Version returns the component version
	Version() string

	// Type returns the component's static constant type
	Type() unikraft.ComponentType

	// Component returns the component's configuration
	Component() ComponentConfig

	// KConfigMenu returns the component's KConfig configuration menu which
	// returns all possible options for the component
	KConfigMenu() (*kconfig.KConfigFile, error)

	// KConfigValeus returns the component's set of file KConfig which is known
	// when the relevant component packages have been retrieved
	KConfigValues() (kconfig.KConfigValues, error)

	// PrintInfo displays the information about the component via the provided
	// iostream
	PrintInfo(*iostreams.IOStreams) error
}

// NameAndVersion accepts a component and provids the canonical string
// representation of the component with its name and version
func NameAndVersion(component Component) string {
	return fmt.Sprintf("%s:%s", component.Name(), component.Version())
}

// ParseComponentConfig parse short syntax for Component configuration
func ParseComponentConfig(name string, props interface{}) (ComponentConfig, error) {
	component := ComponentConfig{}

	if len(name) > 0 {
		component.Name = name
	}

	switch entry := props.(type) {
	case string:
		if strings.Contains(entry, "@") {
			split := strings.Split(entry, "@")
			if len(split) == 2 {
				component.Source = split[0]
				component.Version = split[1]
			}
		} else if f, err := os.Stat(entry); err == nil && f.IsDir() {
			component.Source = entry
		} else if u, err := url.Parse(entry); err == nil && u.Scheme != "" && u.Host != "" {
			component.Source = u.Path
		} else {
			component.Version = entry
		}

	// TODO: This is handled by the transformer, do we really need to do this
	// here?
	case map[string]interface{}:
		for key, prop := range entry {
			switch key {
			case "version":
				component.Version = prop.(string)
			case "source":
				prop := prop.(string)
				if strings.Contains(prop, "@") {
					split := strings.Split(prop, "@")
					if len(split) == 2 {
						component.Version = split[1]
						prop = split[0]
					}
				}

				component.Source = prop

			case "kconfig":
				switch tprop := prop.(type) {
				case map[string]interface{}:
					component.Configuration = kconfig.NewKConfigValuesFromMap(tprop)
				case []interface{}:
					component.Configuration = kconfig.NewKConfigValuesFromSlice(tprop...)
				}
			}
		}
	}

	return component, nil
}

func (cc *ComponentConfig) ApplyOptions(opts ...ComponentOption) error {
	for _, opt := range opts {
		if err := opt(cc); err != nil {
			return err
		}
	}

	return nil
}

func (cc *ComponentConfig) Log() log.Logger {
	return cc.log
}

// Workdir exposes the instantiated component's working directory
func (cc *ComponentConfig) Workdir() string {
	return cc.workdir
}

// SetWorkdir sets the instantiated component's working directory
func (cc *ComponentConfig) SetWorkdir(workdir string) {
	cc.workdir = workdir
}

// SourceDir returns the well-known location of the component given its working
// directory, type and name.
func (cc *ComponentConfig) SourceDir() (string, error) {
	return unikraft.PlaceComponent(
		cc.workdir,
		cc.ctype,
		cc.Name,
	)
}

// IsUnpackedInProject indicates whether the package has been unpacked into a
// project specified by the working directory option
func (cc *ComponentConfig) IsUnpackedInProject() bool {
	local, err := cc.SourceDir()
	if err != nil {
		cc.log.Errorf("could not place component: %v", err)
		return false
	}

	if f, err := os.Stat(local); err == nil && f.IsDir() {
		return true
	}

	return false
}

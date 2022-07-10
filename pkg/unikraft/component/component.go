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

package component

import (
	"context"

	"go.unikraft.io/kit/pkg/log"
	"go.unikraft.io/kit/pkg/pkgmanager"
)

// ComponentConfig is the shared attribute structure provided to all
// microlibraries, whether they are a library, platform, architecture, an
// application itself or the Unikraft core.
type ComponentConfig struct {
	Name          string  `yaml:",omitempty" json:"-"`
	Version       string  `yaml:",omitempty" json:"version,omitempty"`
	Source        string  `yaml:",omitempty" json:"source,omitempty"`
	Configuration KConfig `yaml:",omitempty" json:"kconfig,omitempty"`

	Extensions map[string]interface{} `yaml:",inline" json:"-"`

	coreSource     string // The value of Unikraft's `source:` directive
	packageManager *pkgmanager.PackageManager
	log            log.Logger

	// Context should contain all implementation-specific options, using
	// `context.WithValue`
	ctx context.Context
}

// Component is the abstract interface for managing the individual microlibrary
type Component interface {
	// Name returns the component name
	Name() string

	// Version returns the component version
	Version() string

	// Type returns the component's static constant type
	Type() unikraft.ComponentType
}

// ParseComponentConfig parse short syntax for Component configuration
func ParseComponentConfig(name string, props interface{}) (ComponentConfig, error) {
	component := ComponentConfig{}

	if len(name) > 0 {
		component.Name = name
	}

	switch entry := props.(type) {
	case string:
		component.Version = entry
	
	// TODO: This is handled by the transformer, do we really need to do this
	// here?
	case map[string]interface{}:
		for key, prop := range entry {
			switch key {
			case "version":
				component.Version = prop.(string)
			case "source":
				component.Source = prop.(string)
			// Also handled by the transformer, and the abstraction exists within
			// schema so any new code in this package would be duplicate.
			// case "kconfig":
			// 	component.Configuration = NewKConfig(prop)
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

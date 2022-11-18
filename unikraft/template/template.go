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

// Package template implements the interface through which templates can
// be edited and configured.
package template

import (
	"fmt"
	"os"
	"path/filepath"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

// Template interfaces with the standard component in Unikraft
type Template interface {
	component.Component
}

// TemplateConfig is the configuration of a template. It is identical to
// the component configuration
type TemplateConfig struct {
	component.ComponentConfig
}

// Name returns the name of the template
func (tc TemplateConfig) Name() string {
	return tc.ComponentConfig.Name
}

// Source returns the source of the template
func (tc TemplateConfig) Source() string {
	return tc.ComponentConfig.Source
}

// Version returns the version of the template
func (tc TemplateConfig) Version() string {
	return tc.ComponentConfig.Version
}

// Type returns the type of the template
func (tc TemplateConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

// Component returns the component of the template
func (tc TemplateConfig) Component() component.ComponentConfig {
	return tc.ComponentConfig
}

// KConfigMenu returns the path to the kconfig file of the template
func (tc TemplateConfig) KConfigMenu() (*kconfig.KConfigFile, error) {
	sourceDir, err := tc.ComponentConfig.SourceDir()
	if err != nil {
		return nil, fmt.Errorf("could not get library source directory: %v", err)
	}

	config_uk := filepath.Join(sourceDir, unikraft.Config_uk)
	if _, err := os.Stat(config_uk); err != nil {
		return nil, fmt.Errorf("could not read component Config.uk: %v", err)
	}

	return kconfig.Parse(config_uk)
}

// KConfigValues returns the kconfig values of the template
func (tc TemplateConfig) KConfigValues() (kconfig.KConfigValues, error) {
	menu, err := tc.KConfigMenu()
	if err != nil {
		return nil, fmt.Errorf("could not list KConfig values: %v", err)
	}

	values := kconfig.KConfigValues{}
	values.OverrideBy(tc.Configuration)

	if menu == nil {
		return values, nil
	}

	values.Set(kconfig.Prefix+menu.Root.Name, kconfig.Yes)

	return values, nil
}

// PrintInfo prints information about the template
func (tc TemplateConfig) PrintInfo(io *iostreams.IOStreams) error {
	fmt.Fprint(io.Out, "not implemented: unikraft.lib.LibraryConfig.PrintInfo")
	return nil
}

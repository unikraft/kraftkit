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

package lib

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type Library interface {
	component.Component
}

type LibraryConfig struct {
	component.ComponentConfig
}

type Libraries map[string]LibraryConfig

// Error definitions for common errors used in lib.
var ErrOmittedArchitecture = errors.New("cannot ommit architecture name")

// ParseLibraryConfig parse short syntax for LibraryConfig
func ParseLibraryConfig(version string) (LibraryConfig, error) {
	lib := LibraryConfig{}

	if strings.Contains(version, "@") {
		split := strings.Split(version, "@")
		if len(split) == 2 {
			lib.ComponentConfig.Source = split[0]
			version = split[1]
		}
	}

	if len(version) == 0 {
		return lib, ErrOmittedArchitecture
	}

	lib.ComponentConfig.Version = version

	return lib, nil
}

func (lc LibraryConfig) Name() string {
	return lc.ComponentConfig.Name
}

func (lc LibraryConfig) Source() string {
	return lc.ComponentConfig.Source
}

func (lc LibraryConfig) Version() string {
	return lc.ComponentConfig.Version
}

func (lc LibraryConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeLib
}

func (lc LibraryConfig) Component() component.ComponentConfig {
	return lc.ComponentConfig
}

func (lc LibraryConfig) KConfigMenu() (*kconfig.KConfigFile, error) {
	sourceDir, err := lc.ComponentConfig.SourceDir()
	if err != nil {
		return nil, fmt.Errorf("could not get library source directory: %v", err)
	}

	config_uk := filepath.Join(sourceDir, unikraft.Config_uk)
	if _, err := os.Stat(config_uk); err != nil {
		return nil, fmt.Errorf("could not read component Config.uk: %v", err)
	}

	return kconfig.Parse(config_uk)
}

func (lc LibraryConfig) KConfigValues() (kconfig.KConfigValues, error) {
	menu, err := lc.KConfigMenu()
	if err != nil {
		return nil, fmt.Errorf("could not list KConfig values: %v", err)
	}

	values := kconfig.KConfigValues{}
	values.OverrideBy(lc.Configuration)

	if menu == nil {
		return values, nil
	}

	values.Set(kconfig.Prefix+menu.Root.Name, kconfig.Yes)

	return values, nil
}

func (lc LibraryConfig) PrintInfo(io *iostreams.IOStreams) error {
	fmt.Fprint(io.Out, "not implemented: unikraft.lib.LibraryConfig.PrintInfo")
	return nil
}

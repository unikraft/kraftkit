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

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type Unikraft interface {
	component.Component
}

type UnikraftConfig struct {
	component.ComponentConfig
}

// ParseUnikraftConfig parse short syntax for UnikraftConfig
func ParseUnikraftConfig(version string) (UnikraftConfig, error) {
	core := UnikraftConfig{}

	if strings.Contains(version, "@") {
		split := strings.Split(version, "@")
		if len(split) == 2 {
			core.ComponentConfig.Source = split[0]
			version = split[1]
		}
	}

	if len(version) == 0 {
		return core, fmt.Errorf("cannot use empty string for version or source")
	}

	core.ComponentConfig.Version = version

	return core, nil
}

func (uc UnikraftConfig) Name() string {
	return uc.ComponentConfig.Name
}

func (uc UnikraftConfig) Source() string {
	return uc.ComponentConfig.Source
}

func (uc UnikraftConfig) Version() string {
	return uc.ComponentConfig.Version
}

func (uc UnikraftConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeCore
}

func (uc UnikraftConfig) Component() component.ComponentConfig {
	return uc.ComponentConfig
}

func (uc UnikraftConfig) KConfigMenu() (*kconfig.KConfigFile, error) {
	config_uk := filepath.Join(uc.ComponentConfig.Workdir(), unikraft.Config_uk)
	if _, err := os.Stat(config_uk); err != nil {
		return nil, fmt.Errorf("could not read component Config.uk: %v", err)
	}

	return kconfig.Parse(config_uk)
}

func (uc UnikraftConfig) PrintInfo(io *iostreams.IOStreams) error {
	fmt.Fprint(io.Out, "not implemented: unikraft.core.UnikraftConfig.PrintInfo")
	return nil
}

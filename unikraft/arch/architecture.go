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

package arch

import (
	"fmt"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type Architecture interface {
	component.Component
}

type ArchitectureConfig struct {
	component.ComponentConfig `yaml:"-" json:"-"`
}

// ParseArchitectureConfig parse short syntax for architecture configuration
func ParseArchitectureConfig(value string) (ArchitectureConfig, error) {
	architecture := ArchitectureConfig{}

	if len(value) == 0 {
		return architecture, fmt.Errorf("cannot ommit architecture name")
	}

	architecture.ComponentConfig.Name = value

	return architecture, nil
}

func (ac ArchitectureConfig) Name() string {
	return ac.ComponentConfig.Name
}

func (ac ArchitectureConfig) Version() string {
	return ac.ComponentConfig.Version
}

func (ac ArchitectureConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeArch
}

func (ac ArchitectureConfig) PrintInfo(io *iostreams.IOStreams) error {
	fmt.Fprint(io.Out, "not implemented: unikraft.arch.ArchitectureConfig.PrintInfo")
	return nil
}

func (ac ArchitectureConfig) MarshalYAML() (interface{}, error) {

	return ac.ComponentConfig.Name, nil
}

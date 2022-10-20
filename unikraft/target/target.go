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

package target

import (
	"fmt"

	"kraftkit.sh/initrd"
	"kraftkit.sh/iostreams"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/plat"
)

type TargetConfig struct {
	component.ComponentConfig

	Architecture arch.ArchitectureConfig `yaml:",omitempty" json:"architecture,omitempty"`
	Platform     plat.PlatformConfig     `yaml:",omitempty" json:"platform,omitempty"`
	Format       string                  `yaml:",omitempty" json:"format,omitempty"`
	Kernel       string                  `yaml:",omitempty" json:"kernel,omitempty"`
	KernelDbg    string                  `yaml:",omitempty" json:"kerneldbg,omitempty"`
	Initrd       *initrd.InitrdConfig    `yaml:",omitempty" json:"initrd,omitempty"`
	Command      []string                `yaml:",omitempty" json:"commands"`

	Extensions map[string]interface{} `yaml:",inline" json:"-"`
}

type Targets []TargetConfig

func (tc TargetConfig) Name() string {
	return tc.ComponentConfig.Name
}

func (tc TargetConfig) Source() string {
	return tc.ComponentConfig.Source
}

func (tc TargetConfig) Version() string {
	return tc.ComponentConfig.Version
}

func (tc TargetConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeUnknown
}

func (tc TargetConfig) Component() component.ComponentConfig {
	return tc.ComponentConfig
}

func (tc TargetConfig) KConfigValues() (kconfig.KConfigValues, error) {
	arch, err := tc.Architecture.KConfigValues()
	if err != nil {
		return nil, fmt.Errorf("could not read architecture KConfig values: %v", err)
	}

	plat, err := tc.Platform.KConfigValues()
	if err != nil {
		return nil, fmt.Errorf("could not read platform KConfig values: %v", err)
	}

	values := kconfig.KConfigValues{}
	values.OverrideBy(tc.Configuration)
	values.OverrideBy(arch)
	values.OverrideBy(plat)

	return values, nil
}

func (tc TargetConfig) KConfigMenu() (*kconfig.KConfigFile, error) {
	return nil, fmt.Errorf("target does not have a Config.uk file")
}

// ArchPlatString returns the canonical name for platform architecture string
// combination
func (tc *TargetConfig) ArchPlatString() string {
	return tc.Platform.Name() + "-" + tc.Architecture.Name()
}

func (tc TargetConfig) PrintInfo(io *iostreams.IOStreams) error {
	fmt.Fprint(io.Out, "not implemented: unikraft.plat.TargetConfig.PrintInfo")
	return nil
}

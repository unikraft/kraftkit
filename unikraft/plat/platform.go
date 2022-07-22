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

package plat

import (
	"fmt"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type Platform interface {
	component.Component
}

type PlatformConfig struct {
	component.ComponentConfig
}

// ParsePlatformConfig parse short syntax for platform configuration
func ParsePlatformConfig(value string) (PlatformConfig, error) {
	platform := PlatformConfig{}

	if len(value) == 0 {
		return platform, fmt.Errorf("cannot ommit platform name")
	}

	platform.ComponentConfig.Name = value

	return platform, nil
}

func (pc PlatformConfig) Name() string {
	return pc.ComponentConfig.Name
}

func (pc PlatformConfig) Version() string {
	return pc.ComponentConfig.Version
}

func (pc PlatformConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypePlat
}

func (pc PlatformConfig) PrintInfo(io *iostreams.IOStreams) error {
	fmt.Fprint(io.Out, "not implemented: unikraft.plat.PlatformConfig.PrintInfo")
	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package arch

import (
	"fmt"
	"strings"

	"kraftkit.sh/iostreams"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type Architecture interface {
	component.Component
}

type ArchitectureConfig struct {
	component.ComponentConfig
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

func (ac ArchitectureConfig) Source() string {
	return ac.ComponentConfig.Source
}

func (ac ArchitectureConfig) Version() string {
	return ac.ComponentConfig.Version
}

func (ac ArchitectureConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeArch
}

func (ac ArchitectureConfig) Component() component.ComponentConfig {
	return ac.ComponentConfig
}

func (ac ArchitectureConfig) KConfigMenu() (*kconfig.KConfigFile, error) {
	// Architectures are built directly into the Unikraft core for now.
	return nil, nil
}

func (ac ArchitectureConfig) KConfigValues() (kconfig.KConfigValues, error) {
	values := kconfig.KConfigValues{}
	values.OverrideBy(ac.Configuration)

	// The following are built-in assumptions given the naming conventions used
	// within the Unikraft core.

	var arch strings.Builder
	arch.WriteString(kconfig.Prefix)

	switch ac.Name() {
	case "x86_64", "amd64":
		arch.WriteString("MARCH_X86_64_GENERIC")
	case "arm32":
		arch.WriteString("MARCH_ARM32_CORTEXA7")
	case "arm64":
		arch.WriteString("MCPU_ARM64_NONE")
	default:
		return nil, fmt.Errorf("unknown architecture: %s", ac.Name())
	}

	values.Set(arch.String(), kconfig.Yes)

	return values, nil
}

func (ac ArchitectureConfig) PrintInfo(io *iostreams.IOStreams) error {
	fmt.Fprint(io.Out, "not implemented: unikraft.arch.ArchitectureConfig.PrintInfo")
	return nil
}

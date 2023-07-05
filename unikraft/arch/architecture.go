// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package arch

import (
	"context"
	"fmt"
	"os"
	"strings"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type Architecture interface {
	component.Component
}

type ArchitectureConfig struct {
	// name of the library.
	name string

	// version of the library.
	version string

	// source of the library (can be either remote or local, this attribute is
	// ultimately handled by the packmanager).
	source string

	// path is the location to this library within the context of a project.
	path string

	// kconfig list of kconfig key-values specific to this library.
	kconfig kconfig.KeyValueMap
}

// NewArchitectureFromSchema parse short syntax for architecture configuration
func NewArchitectureFromSchema(value string) (ArchitectureConfig, error) {
	architecture := ArchitectureConfig{}

	if len(value) == 0 {
		return architecture, fmt.Errorf("cannot ommit architecture name")
	}

	architecture.name = value

	return architecture, nil
}

func (ac ArchitectureConfig) Name() string {
	return ac.name
}

func (ac ArchitectureConfig) Source() string {
	return ac.source
}

func (ac ArchitectureConfig) Version() string {
	return ac.version
}

func (ac ArchitectureConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeArch
}

func (ac ArchitectureConfig) Path() string {
	return ac.path
}

func (ac ArchitectureConfig) IsUnpacked() bool {
	if f, err := os.Stat(ac.Path()); err == nil && f.IsDir() {
		return true
	}

	return false
}

func (ac ArchitectureConfig) KConfigTree(context.Context, ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	// Architectures are built directly into the Unikraft core for now.
	return nil, nil
}

func (ac ArchitectureConfig) KConfig() kconfig.KeyValueMap {
	values := kconfig.KeyValueMap{}
	values.OverrideBy(ac.kconfig)

	// The following are built-in assumptions given the naming conventions used
	// within the Unikraft core.

	var arch strings.Builder
	arch.WriteString(kconfig.Prefix)

	switch ac.Name() {
	case "x86_64", "amd64":
		arch.WriteString("ARCH_X86_64")
	case "arm32":
		arch.WriteString("ARCH_ARM_32")
	case "arm64":
		arch.WriteString("ARCH_ARM_64")
	}

	values.Set(arch.String(), kconfig.Yes)

	return values
}

func (ac ArchitectureConfig) MarshalYAML() (interface{}, error) {
	return nil, nil
}

func (ac ArchitectureConfig) PrintInfo(ctx context.Context) string {
	return "not implemented: unikraft.arch.ArchitectureConfig.PrintInfo"
}

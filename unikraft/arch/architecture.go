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

type ArchitectureName string

const (
	ArchitectureUnknown = ArchitectureName("unknown")
	ArchitectureX86_64  = ArchitectureName("x86_64")
	ArchitectureArm64   = ArchitectureName("arm64")
	ArchitectureArm     = ArchitectureName("arm")
)

// String implements fmt.Stringer
func (ht ArchitectureName) String() string {
	return string(ht)
}

// ArchitectureByName returns the architecture for a given name.
// If the name is not known, it returns it unchanged.
func ArchitectureByName(name string) ArchitectureName {
	architectures := ArchitecturesByName()
	if _, ok := architectures[name]; !ok {
		return ArchitectureUnknown
	}
	return architectures[name]
}

// ArchitecturesByName returns the list of known architectures and their name alises.
func ArchitecturesByName() map[string]ArchitectureName {
	return map[string]ArchitectureName{
		"x86_64": ArchitectureX86_64,
		"arm64":  ArchitectureArm64,
		"arm":    ArchitectureArm,
	}
}

// Architectures returns all the unique Architectures.
func Architectures() []ArchitectureName {
	return []ArchitectureName{
		ArchitectureX86_64,
		ArchitectureArm64,
		ArchitectureArm,
	}
}

// ArchitectureAliases returns all the name alises for a given architecture.
func ArchitectureAliases() map[ArchitectureName][]string {
	aliases := map[ArchitectureName][]string{}

	for alias, plat := range ArchitecturesByName() {
		if aliases[plat] == nil {
			aliases[plat] = make([]string, 0)
		}

		aliases[plat] = append(aliases[plat], alias)
	}

	return aliases
}

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

// NewArchitectureFromOptions is a constructor that configures an architecture.
func NewArchitectureFromOptions(opts ...ArchitectureOption) (Architecture, error) {
	pc := ArchitectureConfig{}

	for _, opt := range opts {
		if err := opt(&pc); err != nil {
			return nil, err
		}
	}

	return &pc, nil
}

func (ac ArchitectureConfig) Name() string {
	return ac.name
}

func (ac ArchitectureConfig) String() string {
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

	switch ArchitectureName(ac.Name()) {
	case ArchitectureX86_64:
		arch.WriteString("ARCH_X86_64")
	case ArchitectureArm:
		arch.WriteString("ARCH_ARM_32")
	case ArchitectureArm64:
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

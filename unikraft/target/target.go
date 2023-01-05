// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package target

import (
	"fmt"

	"kraftkit.sh/initrd"
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

func (tc TargetConfig) Path() string {
	return ""
}

func (tc TargetConfig) IsUnpacked() bool {
	return false
}

func (tc TargetConfig) Component() component.ComponentConfig {
	return tc.ComponentConfig
}

func (tc TargetConfig) KConfig() (kconfig.KeyValueMap, error) {
	arch, err := tc.Architecture.KConfig()
	if err != nil {
		return nil, fmt.Errorf("could not read architecture KConfig values: %v", err)
	}

	plat, err := tc.Platform.KConfig()
	if err != nil {
		return nil, fmt.Errorf("could not read platform KConfig values: %v", err)
	}

	values := kconfig.KeyValueMap{}
	values.OverrideBy(tc.Configuration)
	values.OverrideBy(arch)
	values.OverrideBy(plat)

	return values, nil
}

func (tc TargetConfig) KConfigTree(env ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, fmt.Errorf("target does not have a Config.uk file")
}

// ArchPlatString returns the canonical name for platform architecture string
// combination
func (tc *TargetConfig) ArchPlatString() string {
	return tc.Platform.Name() + "-" + tc.Architecture.Name()
}

func (tc TargetConfig) PrintInfo() string {
	return "not implemented: unikraft.plat.TargetConfig.PrintInfo"
}

// TargetName returns the name of the kernel image based on standard pattern
// which is baked within Unikraft's build system, see for example `KVM_IMAGE`.
// If we do not have a target name, return an error.
func KernelName(target TargetConfig) (string, error) {
	if target.ComponentConfig.Name == "" {
		return "", fmt.Errorf("target name not set, cannot determine binary name")
	}

	return fmt.Sprintf(
		"%s_%s-%s",
		target.ComponentConfig.Name,
		target.Platform.Name(),
		target.Architecture.Name(),
	), nil
}

// KernelDbgName is identical to KernelName but is used to access the symbolic
// kernel image which has not been stripped.
func KernelDbgName(target TargetConfig) (string, error) {
	name, err := KernelName(target)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.dbg", name), nil
}

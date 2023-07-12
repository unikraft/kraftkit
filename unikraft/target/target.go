// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package target

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/juju/errors"
	"kraftkit.sh/initrd"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/plat"
)

type Target interface {
	component.Component

	// Architecture is the component architecture for this target.
	Architecture() arch.Architecture

	// Platform is the component platform for this target.
	Platform() plat.Platform

	// Format is the desired package implementation for this target.
	Format() pack.PackageFormat

	// Kernel is the path to the kernel for this target.
	Kernel() string

	// KernelDbg is the path to the symbolic (unstripped) kernel for this target.
	KernelDbg() string

	// Initrd contains the initramfs configuration for this target.
	Initrd() *initrd.InitrdConfig

	// Command is the command-line arguments set for this target.
	Command() []string

	// ConfigFilename returns the target-specific `.config` file which contains
	// all the porclained KConfig key values which is formatted
	// `.config.<TARGET-NAME>`
	ConfigFilename() string
}

type TargetConfig struct {
	// name of the target.
	name string

	// architecture is the target architecture.
	architecture arch.ArchitectureConfig

	// platform is the target platform.
	platform plat.PlatformConfig

	// kconfig list of kconfig key-values specific to this library.
	kconfig kconfig.KeyValueMap

	// format is the desired packaging format.
	format pack.PackageFormat

	// kernel is the path to the kernel for this target.
	kernel string

	// kernelDbg is the path to the symbolic (unstripped) kernel for this target.
	kernelDbg string

	// initrd is the configuration for the initrd.
	initrd *initrd.InitrdConfig

	// command is the command-line arguments set for this target.
	command []string
}

// NewTargetFromOptions is a constructor for TargetConfig.
func NewTargetFromOptions(opts ...TargetOption) (Target, error) {
	tc := TargetConfig{}

	for _, opt := range opts {
		if err := opt(&tc); err != nil {
			return nil, err
		}
	}

	return &tc, nil
}

func (tc *TargetConfig) Name() string {
	return tc.name
}

func (tc *TargetConfig) Source() string {
	return ""
}

func (tc *TargetConfig) Version() string {
	return ""
}

func (tc *TargetConfig) Architecture() arch.Architecture {
	return tc.architecture
}

func (tc *TargetConfig) Platform() plat.Platform {
	return tc.platform
}

func (tc *TargetConfig) Kernel() string {
	return tc.kernel
}

func (tc *TargetConfig) KernelDbg() string {
	return tc.kernelDbg
}

func (tc *TargetConfig) Initrd() *initrd.InitrdConfig {
	return tc.initrd
}

func (tc *TargetConfig) Format() pack.PackageFormat {
	return tc.format
}

func (tc *TargetConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeUnknown
}

func (tc *TargetConfig) Path() string {
	return ""
}

func (tc *TargetConfig) Command() []string {
	return tc.command
}

func (tc *TargetConfig) IsUnpacked() bool {
	return false
}

func (tc *TargetConfig) KConfig() kconfig.KeyValueMap {
	if tc.kconfig == nil {
		tc.kconfig = kconfig.KeyValueMap{}
		tc.kconfig.OverrideBy(tc.architecture.KConfig())
		tc.kconfig.OverrideBy(tc.platform.KConfig())
	}

	return tc.kconfig
}

func (tc *TargetConfig) ConfigFilename() string {
	return fmt.Sprintf("%s.%s", kconfig.DotConfigFileName, filepath.Base(tc.kernel))
}

func (tc *TargetConfig) KConfigTree(_ context.Context, env ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, errors.New("target does not have a Config.uk file")
}

func (tc *TargetConfig) PrintInfo(ctx context.Context) string {
	return "not implemented: unikraft.plat.TargetConfig.PrintInfo"
}

// TargetName returns the name of the kernel image based on standard pattern
// which is baked within Unikraft's build system, see for example `KVM_IMAGE`.
// If we do not have a target name, return an error.
func KernelName(target TargetConfig) (string, error) {
	if target.Name() == "" {
		return "", errors.New("target name not set, cannot determine binary name")
	}

	return fmt.Sprintf(
		"%s_%s-%s",
		target.Name(),
		target.platform.Name(),
		target.architecture.Name(),
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

// TargetPlatArchName returns the canonical name for the platform and
// architecture combination.
func TargetPlatArchName(target Target) string {
	return fmt.Sprintf(
		"%s/%s",
		target.Platform().Name(),
		target.Architecture().Name(),
	)
}

// MarshalYAML makes TargetConfig implement yaml.Marshaller
func (tc TargetConfig) MarshalYAML() (interface{}, error) {
	return map[string]interface{}{
		"architecture": tc.architecture.Name(),
		"platform":     tc.platform.Name(),
	}, nil
}

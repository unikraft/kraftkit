// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package plat

import (
	"context"
	"os"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type Platform interface {
	component.Component
}

type PlatformConfig struct {
	// name of the platform.
	name string

	// version of the platform.
	version string

	// source of the platform (can be either remote or local, this attribute is
	// ultimately handled by the packmanager).
	source string

	// path is the location to this platform within the context of a project.
	path string

	// internal dictates whether the platform comes from the Unikraft core
	// repository.
	internal bool

	// kconfig list of kconfig key-values specific to this platform.
	kconfig kconfig.KeyValueMap
}

// NewPlatformFromOptions is a constructor that configures a platform configuration.
func NewPlatformFromOptions(opts ...PlatformOption) (Platform, error) {
	pc := PlatformConfig{}

	for _, opt := range opts {
		if err := opt(&pc); err != nil {
			return nil, err
		}
	}

	return &pc, nil
}

func (pc PlatformConfig) Name() string {
	return pc.name
}

func (pc PlatformConfig) String() string {
	return pc.name
}

func (pc PlatformConfig) Source() string {
	return pc.source
}

func (pc PlatformConfig) Version() string {
	return pc.version
}

func (pc PlatformConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypePlat
}

func (pc PlatformConfig) Path() string {
	return pc.path
}

func (pc PlatformConfig) IsUnpacked() bool {
	if f, err := os.Stat(pc.Path()); err == nil && f.IsDir() {
		return true
	}

	return false
}

func (pc PlatformConfig) KConfigTree(context.Context, ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	// TODO: Try within the Unikraft codebase as well as via an external
	// microlibrary.  For now, return nil as undetermined.
	return nil, nil
}

func (pc PlatformConfig) KConfig() kconfig.KeyValueMap {
	values := kconfig.KeyValueMap{}
	values.OverrideBy(pc.kconfig)

	// The following are built-in assumptions given the naming conventions used
	// within the Unikraft core.
	switch pc.Name() {
	case "fc", "firecracker", "kraftcloud":
		values.Set("CONFIG_PLAT_KVM", kconfig.Yes)
		values.Set("CONFIG_KVM_VMM_FIRECRACKER", kconfig.Yes)
	case "kvm", "qemu":
		values.Set("CONFIG_PLAT_KVM", kconfig.Yes)
		values.Set("CONFIG_KVM_VMM_QEMU", kconfig.Yes)
	case "xen":
		values.Set("CONFIG_PLAT_XEN", kconfig.Yes)
	case "linuxu":
		values.Set("CONFIG_PLAT_LINUXU", kconfig.Yes)
	}

	return values
}

func (pc PlatformConfig) MarshalYAML() (interface{}, error) {
	return nil, nil
}

func (pc PlatformConfig) PrintInfo(ctx context.Context) string {
	return "not implemented: unikraft.plat.PlatformConfig.PrintInfo"
}

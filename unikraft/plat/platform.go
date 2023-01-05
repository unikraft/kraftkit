// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package plat

import (
	"fmt"
	"os"
	"strings"

	"kraftkit.sh/kconfig"
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

func (pc PlatformConfig) Source() string {
	return pc.ComponentConfig.Source
}

func (pc PlatformConfig) Version() string {
	return pc.ComponentConfig.Version
}

func (pc PlatformConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypePlat
}

func (pc PlatformConfig) IsUnpacked() bool {
	local, err := pc.ComponentConfig.SourceDir()
	if err != nil {
		return false
	}

	if f, err := os.Stat(local); err == nil && f.IsDir() {
		return true
	}

	return false
}

func (pc PlatformConfig) Component() component.ComponentConfig {
	return pc.ComponentConfig
}

func (pc PlatformConfig) KConfigTree(env ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	// TODO: Try within the Unikraft codebase as well as via an external
	// microlibrary.  For now, return nil as undetermined.
	return nil, nil
}

func (pc PlatformConfig) KConfig() (kconfig.KeyValueMap, error) {
	values := kconfig.KeyValueMap{}
	values.OverrideBy(pc.Configuration)

	// The following are built-in assumptions given the naming conventions used
	// within the Unikraft core.  Ultimately, this should be discovered by probing
	// the core or the external microlibrary.

	var plat strings.Builder
	plat.WriteString(kconfig.Prefix)
	plat.WriteString("PLAT_")
	plat.WriteString(strings.ToUpper(pc.Name()))

	values.Set(plat.String(), kconfig.Yes)

	return values, nil
}

func (pc PlatformConfig) PrintInfo() string {
	return "not implemented: unikraft.plat.PlatformConfig.PrintInfo"
}

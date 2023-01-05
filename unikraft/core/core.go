// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type Unikraft interface {
	component.Component
}

type UnikraftConfig struct {
	component.ComponentConfig
}

// ParseUnikraftConfig parse short syntax for UnikraftConfig
func ParseUnikraftConfig(version string) (UnikraftConfig, error) {
	core := UnikraftConfig{}

	if strings.Contains(version, "@") {
		split := strings.Split(version, "@")
		if len(split) == 2 {
			core.ComponentConfig.Source = split[0]
			version = split[1]
		}
	}

	if len(version) == 0 {
		return core, fmt.Errorf("cannot use empty string for version or source")
	}

	core.ComponentConfig.Version = version

	return core, nil
}

func (uc UnikraftConfig) Name() string {
	return uc.ComponentConfig.Name
}

func (uc UnikraftConfig) Source() string {
	return uc.ComponentConfig.Source
}

func (uc UnikraftConfig) Version() string {
	return uc.ComponentConfig.Version
}

func (uc UnikraftConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeCore
}

func (uc UnikraftConfig) Path() string {
	return uc.ComponentConfig.Path
}

func (uc UnikraftConfig) IsUnpacked() bool {
	if f, err := os.Stat(uc.Path()); err == nil && f.IsDir() {
		return true
	}

	return false
}

func (uc UnikraftConfig) Component() component.ComponentConfig {
	return uc.ComponentConfig
}

func (uc UnikraftConfig) KConfigTree(extra ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	config_uk := filepath.Join(uc.Path(), unikraft.Config_uk)
	if _, err := os.Stat(config_uk); err != nil {
		return nil, fmt.Errorf("could not read component Config.uk: %v", err)
	}

	return kconfig.Parse(config_uk, uc.Configuration.Override(extra...).Slice()...)
}

func (uc UnikraftConfig) KConfig() (kconfig.KeyValueMap, error) {
	return uc.Configuration, nil
}

func (uc UnikraftConfig) PrintInfo() string {
	return "not implemented: unikraft.core.UnikraftConfig.PrintInfo"
}

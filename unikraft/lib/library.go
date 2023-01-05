// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type Library interface {
	component.Component
}

type LibraryConfig struct {
	component.ComponentConfig
}

type Libraries map[string]LibraryConfig

// ParseLibraryConfig parse short syntax for LibraryConfig
func ParseLibraryConfig(version string) (LibraryConfig, error) {
	lib := LibraryConfig{}

	if strings.Contains(version, "@") {
		split := strings.Split(version, "@")
		if len(split) == 2 {
			lib.ComponentConfig.Source = split[0]
			version = split[1]
		}
	}

	if len(version) == 0 {
		return lib, fmt.Errorf("cannot use empty string for version or source")
	}

	lib.ComponentConfig.Version = version

	return lib, nil
}

func (lc LibraryConfig) Name() string {
	return lc.ComponentConfig.Name
}

func (lc LibraryConfig) Source() string {
	return lc.ComponentConfig.Source
}

func (lc LibraryConfig) Version() string {
	return lc.ComponentConfig.Version
}

func (lc LibraryConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeLib
}

func (lc LibraryConfig) Component() component.ComponentConfig {
	return lc.ComponentConfig
}

func (lc LibraryConfig) KConfigTree(env ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	sourceDir, err := lc.ComponentConfig.SourceDir()
	if err != nil {
		return nil, fmt.Errorf("could not get library source directory: %v", err)
	}

	config_uk := filepath.Join(sourceDir, unikraft.Config_uk)
	if _, err := os.Stat(config_uk); err != nil {
		return nil, fmt.Errorf("could not read component Config.uk: %v", err)
	}

	kconfigValues, err := lc.KConfig()
	if err != nil {
		return nil, err
	}

	return kconfig.Parse(config_uk, kconfigValues.Override(env...).Slice()...)
}

func (lc LibraryConfig) KConfig() (kconfig.KeyValueMap, error) {
	menu, err := lc.KConfigTree()
	if err != nil {
		return nil, fmt.Errorf("could not list KConfig values: %v", err)
	}

	values := kconfig.KeyValueMap{}
	values.OverrideBy(lc.Configuration)

	if menu == nil {
		return values, nil
	}

	values.Set(kconfig.Prefix+menu.Root.Name, kconfig.Yes)

	return values, nil
}

func (lc LibraryConfig) IsUnpacked() bool {
	local, err := lc.ComponentConfig.SourceDir()
	if err != nil {
		return false
	}

	if f, err := os.Stat(local); err == nil && f.IsDir() {
		return true
	}

	return false
}

func (lc LibraryConfig) PrintInfo() string {
	return "not implemented: unikraft.lib.LibraryConfig.PrintInfo"
}

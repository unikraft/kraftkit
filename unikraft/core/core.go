// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package core

import (
	"fmt"
	"os"
	"path/filepath"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

const (
	CONFIG_UK_PLAT = "plat"
	CONFIG_UK_LIB  = "lib"
	CONFIG         = "support/kconfig"
	CONFIGLIB      = "support/kconfiglib"
)

type Unikraft interface {
	component.Component
}

type UnikraftConfig struct {
	// version of the core.
	version string

	// source of the core (can be either remote or local, this attribute is
	// ultimately handled by the packmanager).
	source string

	// path is the location to this core within the context of a project.
	path string

	// kconfig list of kconfig key-values specific to this core.
	kconfig kconfig.KeyValueMap
}

func (uc UnikraftConfig) Name() string {
	return "unikraft"
}

func (uc UnikraftConfig) Source() string {
	return uc.source
}

func (uc UnikraftConfig) Version() string {
	return uc.version
}

func (uc UnikraftConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeCore
}

func (uc UnikraftConfig) Path() string {
	return uc.path
}

func (uc UnikraftConfig) IsUnpacked() bool {
	if f, err := os.Stat(uc.Path()); err == nil && f.IsDir() {
		return true
	}

	return false
}

func (uc UnikraftConfig) KConfigTree(extra ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	config_uk := filepath.Join(uc.Path(), unikraft.Config_uk)
	if _, err := os.Stat(config_uk); err != nil {
		return nil, fmt.Errorf("could not read component Config.uk: %v", err)
	}

	return kconfig.Parse(config_uk, uc.kconfig.Override(extra...).Slice()...)
}

func (uc UnikraftConfig) KConfig() kconfig.KeyValueMap {
	return uc.kconfig
}

func (uc UnikraftConfig) PrintInfo() string {
	return "not implemented: unikraft.core.UnikraftConfig.PrintInfo"
}

func (uk UnikraftConfig) CONFIG_UK_PLAT() (string, error) {
	return filepath.Join(uk.path, CONFIG_UK_PLAT), nil
}

func (uk UnikraftConfig) CONFIG_UK_LIB() (string, error) {
	return filepath.Join(uk.path, CONFIG_UK_LIB), nil
}

func (uk UnikraftConfig) CONFIG_CONFIG_IN() (string, error) {
	return filepath.Join(uk.path, unikraft.Config_uk), nil
}

func (uk UnikraftConfig) CONFIG() (string, error) {
	return filepath.Join(uk.path, CONFIG), nil
}

func (uk UnikraftConfig) CONFIGLIB() (string, error) {
	return filepath.Join(uk.path, CONFIGLIB), nil
}

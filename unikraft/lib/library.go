// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"fmt"
	"os"
	"path/filepath"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

type Library interface {
	component.Component
}

type LibraryConfig struct {
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

type Libraries map[string]LibraryConfig

func (lc LibraryConfig) Name() string {
	return lc.name
}

func (lc LibraryConfig) Source() string {
	return lc.source
}

func (lc LibraryConfig) Version() string {
	return lc.version
}

func (lc LibraryConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeLib
}

func (lc LibraryConfig) Path() string {
	return lc.path
}

func (lc LibraryConfig) KConfigTree(env ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	config_uk := filepath.Join(lc.Path(), unikraft.Config_uk)
	if _, err := os.Stat(config_uk); err != nil {
		return nil, fmt.Errorf("could not read component Config.uk: %v", err)
	}

	return kconfig.Parse(config_uk, lc.kconfig.Override(env...).Slice()...)
}

func (lc LibraryConfig) KConfig() kconfig.KeyValueMap {
	menu, _ := lc.KConfigTree()

	values := kconfig.KeyValueMap{}
	values.OverrideBy(lc.kconfig)

	if menu == nil {
		return values
	}

	values.Set(kconfig.Prefix+menu.Root.Name, kconfig.Yes)

	return values
}

func (lc LibraryConfig) IsUnpacked() bool {
	if f, err := os.Stat(lc.Path()); err == nil && f.IsDir() {
		return true
	}

	return false
}

func (lc LibraryConfig) PrintInfo() string {
	return "not implemented: unikraft.lib.LibraryConfig.PrintInfo"
}

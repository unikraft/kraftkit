// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package core

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/lib"
)

const (
	CONFIG_UK_PLAT = "plat"
	CONFIG_UK_LIB  = "lib"
	CONFIG         = "support/kconfig"
	CONFIGLIB      = "support/kconfiglib"
)

type Unikraft interface {
	component.Component

	// Libraries returns the application libraries' configurations
	Libraries(ctx context.Context) (lib.Libraries, error)
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

func (uk UnikraftConfig) Libraries(ctx context.Context) (lib.Libraries, error) {
	// Unikraft internal build system recognises internal libraries simply by
	// iterating over the contents of the lib/ dir.  We do the same here.
	config_uk_lib, err := uk.CONFIG_UK_LIB()
	if err != nil {
		return nil, err
	}

	files, err := ioutil.ReadDir(config_uk_lib)
	if err != nil {
		return nil, err
	}

	libs := lib.Libraries{}

	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		more, err := lib.NewFromDir(
			ctx,
			filepath.Join(config_uk_lib, f.Name()),
			lib.WithIsInternal(true),
			lib.WithSource(uk.Source()),
			lib.WithVersion(uk.Version()),
		)
		if err != nil {
			return nil, err
		}

		for _, l := range more {
			libs[l.Name()] = l
		}
	}

	config_uk_plat, err := uk.CONFIG_UK_PLAT()
	if err != nil {
		return nil, err
	}

	files, err = ioutil.ReadDir(config_uk_plat)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		more, err := lib.NewFromDir(
			ctx,
			filepath.Join(config_uk_plat, f.Name()),
			lib.WithIsInternal(true),
			lib.WithSource(uk.Source()),
			lib.WithVersion(uk.Version()),
		)
		if err != nil {
			// Instead of breaking and error-ing out, we simply continue since the
			// plat/ directory (for now, 02/01/23) still contains a common/ and
			// drivers/ directory which is inconsistent with how libs/ is organised.
			// This is an on-going discussion under the topic of "platform
			// re-architecture".
			continue
		}

		for k, l := range more {
			libs[k] = l
		}
	}

	return libs, nil
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

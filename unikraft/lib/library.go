// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"fmt"
	"net/url"
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

// NewLibraryFromSchema is a static
func NewLibraryFromSchema(name string, props interface{}) (LibraryConfig, error) {
	lib := LibraryConfig{}

	if len(name) > 0 {
		lib.ComponentConfig.Name = name
	}

	switch entry := props.(type) {
	case string:
		if strings.Contains(entry, "@") {
			split := strings.Split(entry, "@")
			if len(split) == 2 {
				lib.ComponentConfig.Source = split[0]
				lib.ComponentConfig.Version = split[1]
			}
		} else if f, err := os.Stat(entry); err == nil && f.IsDir() {
			lib.ComponentConfig.Source = entry
		} else if u, err := url.Parse(entry); err == nil && u.Scheme != "" && u.Host != "" {
			lib.ComponentConfig.Source = u.Path
		} else {
			lib.ComponentConfig.Version = entry
		}

	// TODO: This is handled by the transformer, do we really need to do this
	// here?
	case map[string]interface{}:
		for key, prop := range entry {
			switch key {
			case "version":
				lib.ComponentConfig.Version = prop.(string)
			case "source":
				prop := prop.(string)
				if strings.Contains(prop, "@") {
					split := strings.Split(prop, "@")
					if len(split) == 2 {
						lib.ComponentConfig.Version = split[1]
						prop = split[0]
					}
				}

				lib.ComponentConfig.Source = prop

			case "kconfig":
				switch tprop := prop.(type) {
				case map[string]interface{}:
					lib.ComponentConfig.Configuration = kconfig.NewKeyValueMapFromMap(tprop)
				case []interface{}:
					lib.ComponentConfig.Configuration = kconfig.NewKeyValueMapFromSlice(tprop...)
				}
			}
		}
	}

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

func (lc LibraryConfig) Path() string {
	return lc.ComponentConfig.Path
}

func (lc LibraryConfig) KConfigTree(env ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	config_uk := filepath.Join(lc.Path(), unikraft.Config_uk)
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
	if f, err := os.Stat(lc.Path()); err == nil && f.IsDir() {
		return true
	}

	return false
}

func (lc LibraryConfig) PrintInfo() string {
	return "not implemented: unikraft.lib.LibraryConfig.PrintInfo"
}

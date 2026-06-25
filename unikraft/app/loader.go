// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2020 The Compose Specification Authors.
// Copyright 2022 Unikraft GmbH. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"context"
	"fmt"
	"strings"

	interp "github.com/compose-spec/compose-go/interpolation"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"kraftkit.sh/initrd"

	kraftfilev07 "unikraft.com/x/kraftfile"
)

const (
	DefaultConfigFile = ".config"
)

func NewApplicationFromInterface(ctx context.Context, iface map[string]interface{}, popts *ProjectOptions) (Application, error) {
	app := application{}

	if err := Transform(ctx, getSection(iface, "labels"), &app.labels); err != nil {
		return nil, err
	}

	app.path = popts.workdir

	if n, ok := iface["rootfs"]; ok {
		switch n.(type) {
		case string:
			app.rootfs = n.(string)
			app.fsType = initrd.FsTypeCpio
		case interface{}:
			rootfsMap := getSectionMap(iface, "rootfs")
			if rootfsMap == nil {
				return nil, errors.New("rootfs must be a mapping or a string")
			}

			if t, ok := rootfsMap["type"]; ok {
				fsTypeStr, ok := t.(string)
				if !ok {
					return nil, errors.New("rootfs type must be a string")
				}
				app.fsType = kraftfilev07.FsType(strings.ToLower(fsTypeStr))
			} else {
				app.fsType = kraftfilev07.FsTypeCpio
			}

			if s, ok := rootfsMap["source"]; ok {
				sourceStr, ok := s.(string)
				if !ok {
					return nil, errors.New("rootfs source must be a string")
				}
				app.rootfs = sourceStr
			} else {
				return nil, errors.New("rootfs source must be specified")
			}
		default:
			return nil, errors.New("rootfs must be a mapping or a string")
		}
	}

	if romsSectionList := getSectionList(iface, "roms"); romsSectionList != nil {
		for _, rom := range romsSectionList {
			switch v := rom.(type) {
			case string:
				app.roms = append(app.roms, kraftfilev07.FS{
					Source: v,
				})
			case map[string]any, map[any]any:
				var fs kraftfilev07.FS
				if err := Transform(ctx, v, &fs); err != nil {
					return nil, err
				}
				app.roms = append(app.roms, fs)
			default:
				return nil, errors.Errorf("invalid type for rom: %T", v)
			}
		}
	}

	if n, ok := iface["cmd"]; ok {
		switch v := n.(type) {
		case string:
			app.command = []string{v}
		case []interface{}:
			for _, cmd := range v {
				cmdString, ok := cmd.(string)
				if !ok {
					return nil, errors.New("cmd must be a string or a list of strings")
				}
				app.command = append(app.command, cmdString)
			}
		}
	}

	if err := Transform(ctx, getSection(iface, "unikraft"), &app.unikraft); err != nil {
		return nil, err
	}

	if err := Transform(ctx, getSection(iface, "template"), &app.template); err != nil {
		return nil, err
	}

	if err := Transform(ctx, getSection(iface, "volumes"), &app.volumes); err != nil {
		return nil, err
	}

	if err := Transform(ctx, getSection(iface, "runtime"), &app.runtime); err != nil {
		return nil, err
	}

	librariesSectionMap := getSectionMap(iface, "libraries")
	if librariesSectionMap == nil {
		return nil, errors.New("libraries section must be a mapping")
	}
	if err := Transform(ctx, librariesSectionMap, &app.libraries); err != nil {
		return nil, err
	}

	targetsSectionList := getSectionList(iface, "targets")
	if targetsSectionList == nil {
		return nil, errors.New("targets section must be a list")
	}
	if err := Transform(ctx, getSectionList(iface, "targets"), &app.targets); err != nil {
		return nil, err
	}

	if err := Transform(ctx, getSection(iface, "env"), &app.env); err != nil {
		return nil, err
	}

	extensions := getSectionMap(iface, "extensions")
	if len(extensions) > 0 {
		app.extensions = extensions
	}

	return &app, nil
}

func getSection(config map[string]interface{}, key string) interface{} {
	section, ok := config[key]
	if !ok {
		return nil
	}

	return section
}

func getSectionMap(config map[string]interface{}, key string) map[string]interface{} {
	section, ok := config[key]
	if !ok {
		return make(map[string]interface{})
	}

	if _, ok := section.(map[string]interface{}); !ok {
		return nil
	}

	return section.(map[string]interface{})
}

func getSectionList(config map[string]interface{}, key string) []interface{} {
	section, ok := config[key]
	if !ok {
		return make([]interface{}, 0)
	}

	if _, ok := section.([]interface{}); !ok {
		return nil
	}

	return section.([]interface{})
}

func parseConfig(b []byte, popts *ProjectOptions) (map[string]interface{}, error) {
	yml, err := ParseYAML(b)
	if err != nil {
		return nil, err
	}
	if !popts.skipInterpolation {
		return interp.Interpolate(yml, *popts.interpolate)
	}
	return yml, err
}

// ParseYAML reads the bytes from a file, parses the bytes into a mapping
// structure, and returns it.
func ParseYAML(source []byte) (map[string]interface{}, error) {
	var cfg interface{}
	if err := yaml.Unmarshal(source, &cfg); err != nil {
		return nil, err
	}

	cfgMap, ok := cfg.(map[interface{}]interface{})
	if !ok {
		return nil, errors.Errorf("Top-level object must be a mapping")
	}

	converted, err := convertToStringKeysRecursive(cfgMap, "")
	if err != nil {
		return nil, err
	}

	return converted.(map[string]interface{}), nil
}

func formatInvalidKeyError(keyPrefix string, key interface{}) error {
	var location string
	if keyPrefix == "" {
		location = "at top level"
	} else {
		location = fmt.Sprintf("in %s", keyPrefix)
	}

	return errors.Errorf("Non-string key %s: %#v", location, key)
}

// keys need to be converted to strings for jsonschema
func convertToStringKeysRecursive(value interface{}, keyPrefix string) (interface{}, error) {
	if mapping, ok := value.(map[interface{}]interface{}); ok {
		dict := make(map[string]interface{})
		for key, entry := range mapping {
			str, ok := key.(string)
			if !ok {
				return nil, formatInvalidKeyError(keyPrefix, key)
			}

			var newKeyPrefix string
			if keyPrefix == "" {
				newKeyPrefix = str
			} else {
				newKeyPrefix = fmt.Sprintf("%s.%s", keyPrefix, str)
			}

			convertedEntry, err := convertToStringKeysRecursive(entry, newKeyPrefix)
			if err != nil {
				return nil, err
			}

			dict[str] = convertedEntry
		}

		return dict, nil
	}

	if list, ok := value.([]interface{}); ok {
		var convertedList []interface{}
		for index, entry := range list {
			newKeyPrefix := fmt.Sprintf("%s[%d]", keyPrefix, index)
			convertedEntry, err := convertToStringKeysRecursive(entry, newKeyPrefix)
			if err != nil {
				return nil, err
			}

			convertedList = append(convertedList, convertedEntry)
		}

		return convertedList, nil
	}

	return value, nil
}

func groupXFieldsIntoExtensions(dict map[string]interface{}) map[string]interface{} {
	extras := map[string]interface{}{}
	for key, value := range dict {
		if strings.HasPrefix(key, "x-") {
			extras[key] = value
			delete(dict, key)
		}

		if d, ok := value.(map[string]interface{}); ok {
			dict[key] = groupXFieldsIntoExtensions(d)
		}
	}

	if len(extras) > 0 {
		dict["extensions"] = extras
	}
	return dict
}

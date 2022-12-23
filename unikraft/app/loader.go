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
	"fmt"
	"strings"

	interp "github.com/compose-spec/compose-go/interpolation"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/target"
	kraftTemplate "kraftkit.sh/unikraft/template"
)

const (
	DefaultOutputDir  = "build"
	DefaultConfigFile = ".config"
)

func NewApplicationFromInterface(iface map[string]interface{}, popts *ProjectOptions) (*ApplicationConfig, error) {
	var err error
	app := ApplicationConfig{}

	name := ""
	if n, ok := iface["name"]; ok {
		name, ok = n.(string)
		if !ok {
			return nil, errors.New("project name must be a string")
		}
	}

	app.ComponentConfig.Name = name

	outdir := DefaultOutputDir
	if n, ok := iface["outdir"]; ok {
		outdir, ok = n.(string)
		if !ok {
			return nil, errors.New("output directory must be a string")
		}
	}

	if popts.resolvePaths {
		app.outDir = popts.RelativePath(outdir)
	}

	app.unikraft, err = LoadUnikraft(getSection(iface, "unikraft"), popts)
	if err != nil {
		return nil, err
	}

	app.template, err = LoadTemplate(getSection(iface, "template"), popts)
	if err != nil {
		return nil, err
	}

	app.libraries, err = LoadLibraries(getSectionMap(iface, "libraries"), popts)
	if err != nil {
		return nil, err
	}

	app.targets, err = LoadTargets(getSectionList(iface, "targets"), popts)
	if err != nil {
		return nil, err
	}

	extensions := getSectionMap(iface, "extensions")
	if len(extensions) > 0 {
		app.extensions = extensions
	}

	return &app, nil
}

// LoadUnikraft produces a UnikraftConfig from a kraft file Dict the source Dict
// is not validated if directly used.
func LoadUnikraft(source interface{}, popts *ProjectOptions) (core.UnikraftConfig, error) {
	// Populate the unikraft component with shared `ComponentConfig` attributes
	base := map[string]component.ComponentConfig{}
	remap := map[string]interface{}{
		"unikraft": source,
	}
	err := Transform(remap, &base)
	if err != nil {
		return core.UnikraftConfig{}, err
	}

	// Seed the unikraft component with the shared attributes and transform
	uk := core.UnikraftConfig{
		ComponentConfig: base["unikraft"],
	}

	if err := Transform(source, &uk); err != nil {
		return uk, err
	}

	if uk.ComponentConfig.Name == "" {
		uk.ComponentConfig.Name = "unikraft"
	}

	if err := uk.ApplyOptions(append(
		popts.copts,
		component.WithType(unikraft.ComponentTypeCore),
	)...); err != nil {
		return uk, err
	}

	return uk, nil
}

// LoadTemplate produces a TemplateConfig from a kraft file Dict the source Dict
// is not validated if directly used.
func LoadTemplate(source interface{}, popts *ProjectOptions) (kraftTemplate.TemplateConfig, error) {
	base := component.ComponentConfig{}
	dataToParse := make(map[string]interface{})

	switch sourceTransformed := source.(type) {
	case string:
		if strings.Contains(sourceTransformed, "@") {
			split := strings.Split(sourceTransformed, "@")
			if len(split) == 2 {
				dataToParse["source"] = split[0]
				dataToParse["name"] = split[0]
				dataToParse["version"] = split[1]
			}
		} else {
			dataToParse["source"] = sourceTransformed
			dataToParse["name"] = sourceTransformed
		}
	case map[string]interface{}:
		dataToParse = source.(map[string]interface{})
	}

	if err := Transform(dataToParse, &base); err != nil {
		return kraftTemplate.TemplateConfig{}, err
	}

	// Seed the shared attributes
	template := kraftTemplate.TemplateConfig{
		ComponentConfig: base,
	}

	if err := template.ApplyOptions(append(
		popts.copts,
		component.WithType(unikraft.ComponentTypeApp),
	)...); err != nil {
		return template, err
	}

	return template, nil
}

// LoadLibraries produces a LibraryConfig map from a kraft file Dict the source
// Dict is not validated if directly used.
func LoadLibraries(source map[string]interface{}, popts *ProjectOptions) (map[string]lib.LibraryConfig, error) {
	// Populate all library components with shared `ComponentConfig` attributes
	bases := make(map[string]component.ComponentConfig)
	if err := Transform(source, &bases); err != nil {
		return make(map[string]lib.LibraryConfig), err
	}

	libraries := make(map[string]lib.LibraryConfig)
	for name, comp := range bases {
		library := lib.LibraryConfig{}

		if err := Transform(source[name], &library); err != nil {
			return libraries, err
		}

		// Seed the the library components with the shared component attributes
		library.ComponentConfig = comp

		if err := library.ApplyOptions(append(
			popts.copts,
			component.WithType(unikraft.ComponentTypeLib),
		)...); err != nil {
			return libraries, err
		}

		switch {
		case library.ComponentConfig.Name == "":
			library.ComponentConfig.Name = name
		}

		libraries[name] = library
	}

	return libraries, nil
}

// LoadTargets produces a LibraryConfig map from a kraft file Dict the source
// Dict is not validated if directly used. Use Load() to enable validation
func LoadTargets(source []interface{}, popts *ProjectOptions) ([]target.TargetConfig, error) {
	// Populate all target components with shared `ComponentConfig` attributes
	bases := []component.ComponentConfig{}
	if err := Transform(source, &bases); err != nil {
		return []target.TargetConfig{}, err
	}

	// Seed the all library components with the shared attributes
	targets := make([]target.TargetConfig, len(bases))
	for i, targ := range bases {
		targets[i] = target.TargetConfig{
			ComponentConfig: targ,
		}
	}

	for i, target := range targets {
		if err := Transform(source[i], &target); err != nil {
			return targets, err
		}

		if err := target.ApplyOptions(popts.copts...); err != nil {
			return targets, err
		}

		if err := target.Architecture.ApplyOptions(append(
			popts.copts,
			component.WithType(unikraft.ComponentTypeArch),
		)...); err != nil {
			return targets, err
		}

		if err := target.Platform.ApplyOptions(append(
			popts.copts,
			component.WithType(unikraft.ComponentTypePlat),
		)...); err != nil {
			return targets, err
		}

		targets[i] = target
	}

	return targets, nil
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

	return section.(map[string]interface{})
}

func getSectionList(config map[string]interface{}, key string) []interface{} {
	section, ok := config[key]
	if !ok {
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

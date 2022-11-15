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
	"os"
	"path/filepath"
	"strings"

	interp "github.com/compose-spec/compose-go/interpolation"
	"github.com/compose-spec/compose-go/template"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"kraftkit.sh/log"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/schema"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/config"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/target"
	kraftTemplate "kraftkit.sh/unikraft/template"
)

const (
	DefaultOutputDir  = "build"
	DefaultConfigFile = ".config"
)

// LoaderOptions supported by Load
type LoaderOptions struct {
	// Skip schema validation
	SkipValidation bool
	// Skip interpolation
	SkipInterpolation bool
	// Skip normalization
	SkipNormalization bool
	// Resolve paths
	ResolvePaths bool
	// Interpolation options
	Interpolate *interp.Options
	// PackageManager can be injected to each component to allow easy retrieval of
	// the component itself with regard to its source files as well as Unikraft's
	PackageManager *packmanager.PackageManager
	// Set project projectName
	projectName string
	// Indicates when the projectName was imperatively set or guessed from path
	projectNameImperativelySet bool
	// Slice of component options to apply to each loaded component
	componentOptions []component.ComponentOption
	// Access to a general purpose logger
	log log.Logger
}

func (o *LoaderOptions) SetProjectName(name string, imperativelySet bool) {
	o.projectName = normalizeProjectName(name)
	o.projectNameImperativelySet = imperativelySet
}

func (o LoaderOptions) GetProjectName() (string, bool) {
	return o.projectName, o.projectNameImperativelySet
}

// WithSkipValidation sets the LoaderOptions to skip validation when loading
// sections
func WithSkipValidation(opts *LoaderOptions) {
	opts.SkipValidation = true
}

func withLoaderLogger(l log.Logger) func(*LoaderOptions) {
	return func(lopts *LoaderOptions) {
		lopts.log = l
	}
}

func withNamePrecedence(absWorkingDir string, popts *ProjectOptions) func(*LoaderOptions) {
	return func(lopts *LoaderOptions) {
		if popts.Name != "" {
			lopts.SetProjectName(popts.Name, true)
		} else {
			lopts.SetProjectName(filepath.Base(absWorkingDir), false)
		}
	}
}

func withComponentOptions(copts ...component.ComponentOption) func(*LoaderOptions) {
	return func(lopts *LoaderOptions) {
		lopts.componentOptions = copts
	}
}

// Load reads a ConfigDetails and returns a fully loaded configuration
func Load(details config.ConfigDetails, options ...func(*LoaderOptions)) (*ApplicationConfig, error) {
	if len(details.ConfigFiles) < 1 {
		return nil, errors.Errorf("No files specified")
	}

	opts := &LoaderOptions{
		Interpolate: &interp.Options{
			Substitute:      template.Substitute,
			LookupValue:     details.LookupConfig,
			TypeCastMapping: interpolateTypeCastMapping,
		},
	}

	for _, op := range options {
		op(opts)
	}

	opts.componentOptions = append(opts.componentOptions,
		component.WithWorkdir(details.WorkingDir),
		component.WithLogger(opts.log),
	)

	// If we have a set package manager, we can directly inject this to each
	// component.
	if opts.PackageManager != nil {
		opts.componentOptions = append(opts.componentOptions,
			component.WithPackageManager(opts.PackageManager),
		)
	}

	var configs []*config.Config
	for i, file := range details.ConfigFiles {
		configDict := file.Config
		if configDict == nil {
			dict, err := parseConfig(file.Content, opts)
			if err != nil {
				return nil, err
			}
			configDict = dict
			file.Config = dict
			details.ConfigFiles[i] = file
		}

		if !opts.SkipValidation {
			if err := schema.Validate(configDict); err != nil {
				return nil, err
			}
		}

		configDict = groupXFieldsIntoExtensions(configDict)

		cfg, err := loadSections(file.Filename, configDict, details, opts)
		if err != nil {
			return nil, err
		}

		configs = append(configs, cfg)
	}

	model, err := merge(configs)
	if err != nil {
		return nil, err
	}

	projectName, projectNameImperativelySet := opts.GetProjectName()
	model.Name = normalizeProjectName(model.Name)
	if !projectNameImperativelySet && model.Name != "" {
		projectName = model.Name
	}

	if projectName != "" {
		details.Configuration.Set(unikraft.UK_NAME, projectName)
	}

	if len(model.Unikraft.ComponentConfig.Source) > 0 {
		if p, err := os.Stat(model.Unikraft.ComponentConfig.Source); err == nil && p.IsDir() {
			details.Configuration.Set(unikraft.UK_BASE, model.Unikraft.ComponentConfig.Source)
		}
	}

	details.Configuration.OverrideBy(model.Unikraft.Configuration)

	for _, library := range model.Libraries {
		details.Configuration.OverrideBy(library.Configuration)
	}
	project, err := NewApplicationOptions(
		WithWorkingDir(details.WorkingDir),
		WithFilename(model.Filename),
		WithOutDir(model.OutDir),
		WithUnikraft(model.Unikraft),
		WithTemplate(model.Template),
		WithLibraries(model.Libraries),
		WithTargets(model.Targets),
		WithConfiguration(details.Configuration),
		WithExtensions(model.Extensions),
	)
	if err != nil {
		return nil, err
	}

	project.ComponentConfig.Name = projectName

	project.ApplyOptions(append(opts.componentOptions,
		component.WithType(unikraft.ComponentTypeApp),
	)...)

	if !opts.SkipNormalization {
		err = normalize(project, opts.ResolvePaths)
		if err != nil {
			return nil, err
		}
	}

	return project, nil
}

func loadSections(filename string, cfgIface map[string]interface{}, configDetails config.ConfigDetails, opts *LoaderOptions) (*config.Config, error) {
	var err error
	cfg := config.Config{
		Filename: filename,
	}

	name := ""
	if n, ok := cfgIface["name"]; ok {
		name, ok = n.(string)
		if !ok {
			return nil, errors.New("project name must be a string")
		}
	}
	cfg.Name = name

	outdir := DefaultOutputDir
	if n, ok := cfgIface["outdir"]; ok {
		outdir, ok = n.(string)
		if !ok {
			return nil, errors.New("output directory must be a string")
		}
	}

	if opts.ResolvePaths {
		cfg.OutDir = configDetails.RelativePath(outdir)
	}

	cfg.Unikraft, err = LoadUnikraft(getSection(cfgIface, "unikraft"), opts)
	if err != nil {
		return nil, err
	}

	cfg.Template, err = LoadTemplate(getSection(cfgIface, "template"), opts)
	if err != nil {
		return nil, err
	}

	cfg.Libraries, err = LoadLibraries(getSectionMap(cfgIface, "libraries"), opts)
	if err != nil {
		return nil, err
	}

	cfg.Targets, err = LoadTargets(getSectionList(cfgIface, "targets"), configDetails, cfg.OutDir, opts)
	if err != nil {
		return nil, err
	}

	extensions := getSectionMap(cfgIface, "extensions")
	if len(extensions) > 0 {
		cfg.Extensions = extensions
	}

	return &cfg, nil
}

// LoadUnikraft produces a UnikraftConfig from a kraft file Dict the source Dict
// is not validated if directly used. Use Load() to enable validation
func LoadUnikraft(source interface{}, opts *LoaderOptions) (core.UnikraftConfig, error) {
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
		opts.componentOptions,
		component.WithType(unikraft.ComponentTypeCore),
	)...); err != nil {
		return uk, err
	}

	return uk, nil
}

// LoadTemplate produces a TemplateConfig from a kraft file Dict the source Dict
// is not validated if directly used. Use Load() to enable validation
func LoadTemplate(source interface{}, opts *LoaderOptions) (kraftTemplate.TemplateConfig, error) {
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
		opts.componentOptions,
		component.WithType(unikraft.ComponentTypeApp),
	)...); err != nil {
		return template, err
	}

	return template, nil
}

// LoadLibraries produces a LibraryConfig map from a kraft file Dict the source
// Dict is not validated if directly used. Use Load() to enable validation
func LoadLibraries(source map[string]interface{}, opts *LoaderOptions) (map[string]lib.LibraryConfig, error) {
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
			opts.componentOptions,
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
func LoadTargets(source []interface{}, configDetails config.ConfigDetails, outdir string, opts *LoaderOptions) ([]target.TargetConfig, error) {
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

	projectName, _ := opts.GetProjectName()

	for i, target := range targets {
		if err := Transform(source[i], &target); err != nil {
			return targets, err
		}

		if err := target.ApplyOptions(opts.componentOptions...); err != nil {
			return targets, err
		}

		if err := target.Architecture.ApplyOptions(append(
			opts.componentOptions,
			component.WithType(unikraft.ComponentTypeArch),
		)...); err != nil {
			return targets, err
		}

		if err := target.Platform.ApplyOptions(append(
			opts.componentOptions,
			component.WithType(unikraft.ComponentTypePlat),
		)...); err != nil {
			return targets, err
		}

		if target.ComponentConfig.Name == "" {
			target.ComponentConfig.Name = projectName
		}

		if target.Kernel == "" {
			target.Kernel = filepath.Join(outdir, fmt.Sprintf(
				// The filename pattern below is a baked in assumption within Unikraft's
				// build system, see for example `KVM_IMAGE`.  TODO: This format should
				// likely be upstreamed into the core as a generic for all platforms.
				"%s_%s-%s",
				projectName,
				target.Platform.Name(),
				target.Architecture.Name(),
			))
		}

		if target.KernelDbg == "" {
			// Another baked-in assumption from the Unikraft build system.  See for
			// example `KVM_DEBUG_IMAGE` which simply makes the same suffix appendage.
			// TODO: As above, this should likely be upstreamed as a generic.
			target.KernelDbg = fmt.Sprintf("%s.dbg", target.Kernel)
		}

		if opts.ResolvePaths {
			target.Kernel = configDetails.RelativePath(target.Kernel)
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

func parseConfig(b []byte, opts *LoaderOptions) (map[string]interface{}, error) {
	yml, err := ParseYAML(b)
	if err != nil {
		return nil, err
	}
	if !opts.SkipInterpolation {
		return interp.Interpolate(yml, *opts.Interpolate)
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

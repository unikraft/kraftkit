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
	"reflect"
	"strings"

	"github.com/mattn/go-shellwords"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"kraftkit.sh/initrd"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/component"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

// TransformerFunc defines a function to perform the actual transformation
type TransformerFunc func(interface{}) (interface{}, error)

// Transformer defines a map to type transformer
type Transformer struct {
	TypeOf reflect.Type
	Func   TransformerFunc
}

// Transform converts the source into the target struct with compose types
// transformer and the specified transformers if any.
func Transform(source interface{}, target interface{}, additionalTransformers ...Transformer) error {
	data := mapstructure.Metadata{}
	config := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			createTransformHook(additionalTransformers...),
			mapstructure.StringToTimeDurationHookFunc(),
		),
		Result:     target,
		Metadata:   &data,
		ZeroFields: false,
		MatchName: func(mapKey, fieldName string) bool {
			maps := map[string]string{
				"kconfig": "Configuration",
			}

			if f, ok := maps[mapKey]; ok && f == fieldName {
				return true
			} else if mapKey == strings.ToLower(fieldName) {
				return true
			}

			return false
		},
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(source)
}

func createTransformHook(additionalTransformers ...Transformer) mapstructure.DecodeHookFuncType {
	transforms := map[reflect.Type]func(interface{}) (interface{}, error){
		reflect.TypeOf(map[string]string{}):       transformMapStringString,
		reflect.TypeOf(kconfig.KConfigValues{}):   transformKConfig,
		reflect.TypeOf(target.Command{}):          transformCommand,
		reflect.TypeOf([]target.TargetConfig{}):   transformTarget,
		reflect.TypeOf(arch.ArchitectureConfig{}): transformArchitecture,
		reflect.TypeOf(plat.PlatformConfig{}):     transformPlatform,
		reflect.TypeOf(initrd.InitrdConfig{}):     transformInitrd,
		reflect.TypeOf(lib.LibraryConfig{}):       transformLibrary,
		reflect.TypeOf(core.UnikraftConfig{}):     transformUnikraft,
		// Use a map as we need to access the name (which is the key)
		reflect.TypeOf(map[string]component.ComponentConfig{}): transformComponents,
	}

	for _, transformer := range additionalTransformers {
		transforms[transformer.TypeOf] = transformer.Func
	}

	return func(_ reflect.Type, target reflect.Type, data interface{}) (interface{}, error) {
		transform, ok := transforms[target]
		if !ok {
			return data, nil
		}
		return transform(data)
	}
}

func toString(value interface{}, allowNil bool) interface{} {
	switch {
	case value != nil:
		return fmt.Sprint(value)
	case allowNil:
		return nil
	default:
		return ""
	}
}

func toMapStringString(value map[string]interface{}, allowNil bool) map[string]interface{} {
	output := make(map[string]interface{})
	for key, value := range value {
		output[key] = toString(value, allowNil)
	}
	return output
}

var transformMapStringString TransformerFunc = func(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case map[string]interface{}:
		return toMapStringString(value, false), nil
	case map[string]string:
		return value, nil
	default:
		return data, errors.Errorf("invalid type %T for map[string]string", value)
	}
}

var transformArchitecture TransformerFunc = func(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case map[string]interface{}:
		return toMapStringString(value, false), nil
	case map[string]string:
		return value, nil
	case string:
		return arch.ParseArchitectureConfig(value)
	default:
		return data, errors.Errorf("invalid type %T for architecture", value)
	}
}

var transformPlatform TransformerFunc = func(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case map[string]interface{}:
		return toMapStringString(value, false), nil
	case map[string]string:
		return value, nil
	case string:
		return plat.ParsePlatformConfig(value)
	default:
		return data, errors.Errorf("invalid type %T for platform", value)
	}
}

var transformInitrd TransformerFunc = func(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case map[string]interface{}:
		if format, ok := value["format"]; ok {
			if typ, ok := initrd.NameToType[format.(string)]; ok {
				value["format"] = typ
			} else {
				return value, errors.Errorf("invalid option for initrd type: %s", format)
			}
		}

		return value, nil
	default:
		return data, errors.Errorf("invalid type %T for platform", value)
	}
}

func transformMappingOrList(mappingOrList interface{}, sep string, allowNil bool) (interface{}, error) {
	switch value := mappingOrList.(type) {
	case map[string]interface{}:
		return toMapStringString(value, allowNil), nil
	case []interface{}:
		result := make(map[string]interface{})
		for _, value := range value {
			key, val := transformValueToMapEntry(value.(string), sep, allowNil)
			result[key] = val
		}
		return result, nil
	}
	return nil, errors.Errorf("expected a map or a list, got %T: %#v", mappingOrList, mappingOrList)
}

func transformValueToMapEntry(value string, separator string, allowNil bool) (string, interface{}) {
	parts := strings.SplitN(value, separator, 2)
	key := parts[0]
	switch {
	case len(parts) == 1 && allowNil:
		return key, nil
	case len(parts) == 1 && !allowNil:
		return key, ""
	default:
		return key, parts[1]
	}
}

var transformCommand TransformerFunc = func(value interface{}) (interface{}, error) {
	if str, ok := value.(string); ok {
		return shellwords.Parse(str)
	}
	return value, nil
}

var transformTarget TransformerFunc = func(data interface{}) (interface{}, error) {
	switch entries := data.(type) {
	case []interface{}:
		// We process the list instead of individual items here.
		// The reason is that one entry might be mapped to multiple TargetConfig.
		// Therefore we take an input of a list and return an output of a list.
		var targets []interface{}
		for _, entry := range entries {
			switch value := entry.(type) {
			case map[string]interface{}:
				targets = append(targets, groupXFieldsIntoExtensions(value))
			default:
				return data, errors.Errorf("invalid type %T for target", value)
			}
		}
		return targets, nil
	default:
		return data, errors.Errorf("invalid type %T for target", entries)
	}
}

var transformComponents TransformerFunc = func(data interface{}) (interface{}, error) {
	switch entries := data.(type) {
	case map[string]interface{}:
		components := make(map[string]interface{}, len(entries))
		for name, props := range entries {
			switch props.(type) {
			case string, map[string]interface{}:
				comp, err := component.ParseComponentConfig(name, props)
				if err != nil {
					return nil, err
				}
				components[name] = comp

			default:
				return data, errors.Errorf("invalid type %T for component", props)
			}
		}

		return components, nil
	}

	return data, errors.Errorf("invalid type %T for component map", data)
}

var transformLibrary TransformerFunc = func(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case map[string]interface{}:
		return toMapStringString(value, false), nil
	case map[string]string:
		return value, nil
	case string:
		return lib.ParseLibraryConfig(value)
	}

	return data, errors.Errorf("invalid type %T for library", data)
}

var transformUnikraft TransformerFunc = func(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case map[string]interface{}:
		return toMapStringString(value, false), nil
	case map[string]string:
		return value, nil
	case string:
		return core.ParseUnikraftConfig(value)
	}

	return data, errors.Errorf("invalid type %T for unikraft", data)
}

var transformKConfig TransformerFunc = func(data interface{}) (interface{}, error) {
	config, err := transformMappingOrList(data, "=", true)
	if err != nil {
		return nil, err
	}

	kconf := kconfig.KConfigValues{}

	for k, v := range config.(map[string]string) {
		kconf.Set(k, v)
	}

	return kconf, nil
}

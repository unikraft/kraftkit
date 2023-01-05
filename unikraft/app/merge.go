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
	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"kraftkit.sh/unikraft/core"
	"kraftkit.sh/unikraft/lib"
	"kraftkit.sh/unikraft/target"
)

func MergeApplicationConfigs(apps []*ApplicationConfig) (*ApplicationConfig, error) {
	base := apps[0]
	for _, override := range apps[1:] {
		var err error
		base.name = mergeNames(base.name, override.name)

		base.unikraft, err = mergeUnikraft(base.unikraft, override.unikraft)
		if err != nil {
			return base, errors.Wrapf(err, "cannot merge services from %s", override.filename)
		}

		base.libraries, err = mergeLibraries(base.libraries, override.libraries)
		if err != nil {
			return base, errors.Wrapf(err, "cannot merge volumes from %s", override.filename)
		}

		base.targets, err = mergeTargets(base.targets, override.targets)
		if err != nil {
			return base, errors.Wrapf(err, "cannot merge networks from %s", override.filename)
		}

		base.extensions, err = mergeExtensions(base.extensions, override.extensions)
		if err != nil {
			return base, errors.Wrapf(err, "cannot merge extensions from %s", override.filename)
		}
	}
	return base, nil
}

func mergeNames(base, override string) string {
	if override != "" {
		return override
	}
	return base
}

func mergeUnikraft(base, override core.UnikraftConfig) (core.UnikraftConfig, error) {
	err := mergo.Merge(&base, &override, mergo.WithOverride)
	return base, err
}

func mergeLibraries(base, override lib.Libraries) (lib.Libraries, error) {
	err := mergo.Map(&base, &override, mergo.WithOverride)
	return base, err
}

func mergeTargets(base, override []target.TargetConfig) ([]target.TargetConfig, error) {
	err := mergo.Merge(&base, &override, mergo.WithOverride)
	return base, err
}

func mergeExtensions(base, override map[string]interface{}) (map[string]interface{}, error) {
	if base == nil {
		base = map[string]interface{}{}
	}
	err := mergo.Map(&base, &override, mergo.WithOverride)
	return base, err
}

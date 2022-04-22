// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2020 The Compose Specification Authors.
// Copyright 2022 Unikraft UG. All rights reserved.
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

package loader

import (
	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"go.unikraft.io/kit/pkg/unikraft/target"
	"go.unikraft.io/kit/schema/types"
)

func merge(configs []*types.Config) (*types.Config, error) {
	base := configs[0]
	for _, override := range configs[1:] {
		var err error
		base.Name = mergeNames(base.Name, override.Name)

		base.Unikraft, err = mergeUnikraft(base.Unikraft, override.Unikraft)
		if err != nil {
			return base, errors.Wrapf(err, "cannot merge services from %s", override.Filename)
		}

		base.Libraries, err = mergeLibraries(base.Libraries, override.Libraries)
		if err != nil {
			return base, errors.Wrapf(err, "cannot merge volumes from %s", override.Filename)
		}

		base.Targets, err = mergeTargets(base.Targets, override.Targets)
		if err != nil {
			return base, errors.Wrapf(err, "cannot merge networks from %s", override.Filename)
		}

		base.Extensions, err = mergeExtensions(base.Extensions, override.Extensions)
		if err != nil {
			return base, errors.Wrapf(err, "cannot merge extensions from %s", override.Filename)
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

func mergeUnikraft(base, override types.UnikraftConfig) (types.UnikraftConfig, error) {
	err := mergo.Merge(&base, &override, mergo.WithOverride)
	return base, err
}

func mergeLibraries(base, override map[string]types.LibraryConfig) (map[string]types.LibraryConfig, error) {
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

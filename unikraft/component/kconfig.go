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

package component

import "strings"

// KConfig is a mapping type that can be converted from a list of
// key[=value] strings. For the key with an empty value (`key=`), the mapped
// value is set to a pointer to `""`. For the key without value (`key`), the
// mapped value is set to nil.
type KConfig map[string]*string

// NewKConfig build a new Mapping from a set of KEY=VALUE strings
func NewKConfig(values []string) KConfig {
	mapping := KConfig{}
	for _, env := range values {
		tokens := strings.SplitN(env, "=", 2)
		if len(tokens) > 1 {
			mapping[tokens[0]] = &tokens[1]
		} else {
			mapping[env] = nil
		}
	}
	return mapping
}

// OverrideBy update KConfig with values from another
// KConfig
func (e KConfig) OverrideBy(other KConfig) KConfig {
	for k, v := range other {
		e[k] = v
	}
	return e
}

// Resolve update a KConfig for keys without value (`key`, but not
// `key=`)
func (e KConfig) Resolve(lookupFn func(string) (string, bool)) KConfig {
	for k, v := range e {
		if v == nil {
			if value, ok := lookupFn(k); ok {
				e[k] = &value
			}
		}
	}
	return e
}

// RemoveEmpty excludes keys that are not associated with a value
func (e KConfig) RemoveEmpty() KConfig {
	for k, v := range e {
		if v == nil {
			delete(e, k)
		}
	}
	return e
}

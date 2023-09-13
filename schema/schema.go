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

package schema

import (
	"errors"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// Validate uses the cueschema to validate the configuration
func Validate(config map[string]interface{}) error {
	ctx := cuecontext.New()
	dataLoader := ctx.Encode(config) // could need to change

	// loading kraft-spec-v0.5.cue
	entrypoints := []string{"kraft-spec-v0.5.cue"}
	bis := load.Instances(entrypoints, nil)

	insts, err := ctx.BuildInstances(bis)
	if err != nil {
		return err
	}
	if len(insts) != 1 {
		return errors.New("more than one instance created for the schema")
	}

	schemaLoader := insts[0]
	schemaLoader = schemaLoader.LookupPath(cue.ParsePath("#KraftSpec"))
	unified := schemaLoader.Unify(dataLoader)
	opts := []cue.Option{
		cue.Attributes(true),
		cue.Definitions(true),
		cue.Hidden(true),
	}
	err = unified.Validate(opts...)
	if err != nil {
		return err
	}
	return nil
}

type SchemaVersion string

const (
	SchemaVersionV0_5   = "0.5"
	SchemaVersionLatest = SchemaVersionV0_5
)

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
	"context"
	"fmt"
	"strings"

	// Enable support for embedded static resources
	_ "embed"

	"github.com/Masterminds/semver/v3"
	"github.com/xeipuuv/gojsonschema"
	"kraftkit.sh/log"
)

// Schema is the Kraftfile specification in JSON schema.
var (
	//go:embed v0.5.json
	SchemaV_05 string

	//go:embed v0.6.json
	SchemaV_06 string
)

// Validate uses the jsonschema to validate the configuration
func Validate(ctx context.Context, config map[string]interface{}) error {
	var spec string
	if s, ok := config["specification"].(string); ok {
		spec = s
	} else if s, ok := config["spec"].(string); ok {
		spec = s
	} else {
		return fmt.Errorf("missing 'spec' version attribute")
	}

	specVer, err := semver.NewVersion(spec)
	if err != nil {
		return fmt.Errorf("could not parse specification version: %w", err)
	}

	latestVer := semver.MustParse(string(SchemaVersionLatest))

	if specVer.LessThan(latestVer) {
		log.G(ctx).Warnf("specification in Kraftfile (v%s) version is not latest (v%s)", spec, SchemaVersionLatest)
	}

	schemaLoader := gojsonschema.NewStringLoader(SchemaV_06)
	dataLoader := gojsonschema.NewGoLoader(config)

	result, err := gojsonschema.Validate(schemaLoader, dataLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		return toError(result)
	}

	return nil
}

const (
	jsonschemaOneOf = "number_one_of"
	jsonschemaAnyOf = "number_any_of"
)

func humanReadableType(definition string) string {
	if definition[0:1] == "[" {
		allTypes := strings.Split(definition[1:len(definition)-1], ",")
		for i, t := range allTypes {
			allTypes[i] = humanReadableType(t)
		}
		return fmt.Sprintf(
			"%s or %s",
			strings.Join(allTypes[0:len(allTypes)-1], ", "),
			allTypes[len(allTypes)-1],
		)
	}
	if definition == "object" {
		return "mapping"
	}
	if definition == "array" {
		return "list"
	}
	return definition
}

func getDescription(err validationError) string {
	switch err.parent.Type() {
	case "invalid_type":
		if expectedType, ok := err.parent.Details()["expected"].(string); ok {
			return fmt.Sprintf("must be a %s", humanReadableType(expectedType))
		}

	case jsonschemaOneOf, jsonschemaAnyOf:
		if err.child == nil {
			return err.parent.Description()
		}

		return err.child.Description()
	}

	return err.parent.Description()
}

type validationError struct {
	parent gojsonschema.ResultError
	child  gojsonschema.ResultError
}

func (err validationError) Error() string {
	description := getDescription(err)
	return fmt.Sprintf("%s %s", err.parent.Field(), description)
}

func toError(result *gojsonschema.Result) error {
	err := getMostSpecificError(result.Errors())
	return err
}

func getMostSpecificError(errors []gojsonschema.ResultError) validationError {
	mostSpecificError := 0
	for i, err := range errors {
		if specificity(err) > specificity(errors[mostSpecificError]) {
			mostSpecificError = i
			continue
		}

		if specificity(err) == specificity(errors[mostSpecificError]) {
			// Invalid type errors win in a tie-breaker for most specific field name
			if err.Type() == "invalid_type" && errors[mostSpecificError].Type() != "invalid_type" {
				mostSpecificError = i
			}
		}
	}

	if mostSpecificError+1 == len(errors) {
		return validationError{parent: errors[mostSpecificError]}
	}

	switch errors[mostSpecificError].Type() {
	case "number_one_of", "number_any_of":
		return validationError{
			parent: errors[mostSpecificError],
			child:  errors[mostSpecificError+1],
		}
	default:
		return validationError{parent: errors[mostSpecificError]}
	}
}

func specificity(err gojsonschema.ResultError) int {
	return len(strings.Split(err.Field(), "."))
}

type SchemaVersion string

const (
	SchemaVersionV0_5   = SchemaVersion("0.5")
	SchemaVersionV0_6   = SchemaVersion("0.6")
	SchemaVersionLatest = SchemaVersionV0_6
)

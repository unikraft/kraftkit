// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package create

import "fmt"

// RolloutQualifier is the detection mechanism used to determine whether the
// specific instance should be affected by the rollout strategy.
type RolloutQualifier string

const (
	// The 'image' qualifier is used to capture instances which have the same
	// image (ignoring digest).
	RolloutQualifierImageName = RolloutQualifier("image")

	// The 'name' qualifier is used to capture instances which have the same name.
	RolloutQualifierInstanceName = RolloutQualifier("name")

	// The 'all' qualifier matches all instances in the service.
	RolloutQualifierAll = RolloutQualifier("all")

	// The 'none' qualifier prevents matching any instances in the service.
	RolloutQualifierNone = RolloutQualifier("none")
)

var _ fmt.Stringer = (*RolloutQualifier)(nil)

// String implements fmt.Stringer
func (strategy RolloutQualifier) String() string {
	return string(strategy)
}

// RolloutQualifiers returns the list of possible rollout qualifier.
func RolloutQualifiers() []RolloutQualifier {
	return []RolloutQualifier{
		RolloutQualifierImageName,
		RolloutQualifierInstanceName,
		RolloutQualifierAll,
		RolloutQualifierNone,
	}
}

// RolloutQualifierNames returns the string representation of all possible
// rollout qualifiers.
func RolloutQualifierNames() []string {
	qualifiers := []string{}
	for _, strategy := range RolloutQualifiers() {
		qualifiers = append(qualifiers, strategy.String())
	}

	return qualifiers
}

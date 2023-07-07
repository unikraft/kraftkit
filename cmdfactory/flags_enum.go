// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package cmdfactory

import (
	"strings"

	"github.com/juju/errors"
)

type EnumFlag struct {
	Allowed []string
	Value   string
}

// NewEnumFlag give a list of allowed flag parameters, where the second argument
// is the default
func NewEnumFlag(allowed []string, d string) *EnumFlag {
	return &EnumFlag{
		Allowed: allowed,
		Value:   d,
	}
}

func (a *EnumFlag) String() string {
	return a.Value
}

func (a *EnumFlag) Set(p string) error {
	isIncluded := func(opts []string, val string) bool {
		for _, opt := range opts {
			if val == opt {
				return true
			}
		}

		return false
	}

	if !isIncluded(a.Allowed, p) {
		return errors.Errorf("%s is not included in: %s", p, strings.Join(a.Allowed, ", "))
	}

	a.Value = p
	return nil
}

func (a *EnumFlag) Type() string {
	return "string"
}

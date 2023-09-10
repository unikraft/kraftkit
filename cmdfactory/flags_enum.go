// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package cmdfactory

import (
	"fmt"
	"strings"
)

type EnumFlag[T fmt.Stringer] struct {
	Allowed []T
	Value   T
}

// NewEnumFlag give a list of allowed flag parameters, where the second argument
// is the default
func NewEnumFlag[T fmt.Stringer](allowed []T, d T) *EnumFlag[T] {
	return &EnumFlag[T]{
		Allowed: allowed,
		Value:   d,
	}
}

func (a *EnumFlag[T]) String() string {
	return a.Value.String()
}

func (a *EnumFlag[T]) Set(p string) error {
	isIncluded := func(opts []T, val string) (bool, *T) {
		for _, opt := range opts {
			if val == opt.String() {
				return true, &opt
			}
		}

		return false, nil
	}

	ok, t := isIncluded(a.Allowed, p)
	if !ok {
		allowed := make([]string, len(a.Allowed))
		for i := range a.Allowed {
			allowed[i] = a.Allowed[i].String()
		}
		return fmt.Errorf("%s is not included in: %s", p, strings.Join(allowed, ", "))
	}

	a.Value = *t
	return nil
}

func (a *EnumFlag[T]) Type() string {
	return "string"
}

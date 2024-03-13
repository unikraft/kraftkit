// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ukargparse

import (
	"fmt"
	"strings"
)

type Params []Param

// Parse accepts varadic length position argument args which represent Unikraft
// command-line arguments and returns structured Params.
func Parse(args ...string) (Params, error) {
	params := []Param{}

	for _, arg := range args {
		nameAndValue := strings.SplitN(arg, "=", 2)
		if len(nameAndValue) != 2 {
			return nil, fmt.Errorf("expected param to be in the format 'libname.param=value' but got: '%s'", arg)
		}

		libAndName := strings.Split(nameAndValue[0], ".")
		if len(libAndName) != 2 {
			return nil, fmt.Errorf("expected param to be in the format 'libname.param=value' but got: '%s'", arg)
		}

		param := paramStr{
			library: libAndName[0],
			name:    libAndName[1],
			value:   nameAndValue[1],
		}

		params = append(params, &param)
	}

	return params, nil
}

// Strings returns all parameters and their string representation which is ready
// to be accepted by Unikraft.
func (params Params) Strings() []string {
	ret := make([]string, len(params))
	for i, param := range params {
		ret[i] = param.String()
	}
	return ret
}

// Contains checks whether the provided needle exists within the existing set of
// Params.
func (params Params) Contains(needle Param) bool {
	for _, param := range params {
		if param.Name() == needle.Name() {
			return true
		}
	}

	return false
}

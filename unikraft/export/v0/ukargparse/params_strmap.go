// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ukargparse

import (
	"fmt"
	"strings"
)

type paramStrMap struct {
	library string
	name    string
	values  map[string]string
}

// ParamStr instantiates a new Param based on a string value.
func ParamStrMap(lib string, name string, values any) Param {
	return &paramStrMap{
		library: lib,
		name:    name,
		values:  values.(map[string]string),
	}
}

// Name implements Param
func (param *paramStrMap) Name() string {
	return fmt.Sprintf("%s.%s", param.library, param.name)
}

// Set implements Param
func (param *paramStrMap) Set(value any) {
	v, ok := value.(map[string]string)
	if !ok {
		return
	}
	param.values = v
}

// Value implements Param
func (param *paramStrMap) Value() any {
	return param.values
}

// WithValue implements Param
func (param *paramStrMap) WithValue(value any) Param {
	param.Set(value)
	return param
}

// String implements Param
func (param *paramStrMap) String() string {
	var ret strings.Builder
	ret.WriteString(param.library)
	ret.WriteString(".")
	ret.WriteString(param.name)
	ret.WriteString("[")

	for k, v := range param.values {
		ret.WriteString(k)
		ret.WriteString("=")
		ret.WriteString(v)
	}

	ret.WriteString("]")

	return ret.String()
}

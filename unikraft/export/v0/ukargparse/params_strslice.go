// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ukargparse

import (
	"fmt"
	"strings"
)

type ParamStrSlice struct {
	library string
	name    string
	values  []string
}

// NewParamStrSlice instantiates a new Param based on a slice of strings.
func NewParamStrSlice(lib string, name string, values any) Param {
	param := &ParamStrSlice{
		library: lib,
		name:    name,
	}

	if strs, ok := values.([]string); ok {
		param.values = strs
	}

	return param
}

// Name implements Param
func (param *ParamStrSlice) Name() string {
	return fmt.Sprintf("%s.%s", param.library, param.name)
}

// Set implements Param
func (param *ParamStrSlice) Set(value any) {
	v, ok := value.([]string)
	if !ok {
		return
	}

	param.values = v
}

// Value implements Param
func (param *ParamStrSlice) Value() any {
	return param.values
}

// WithValue implements Param
func (param *ParamStrSlice) WithValue(value any) Param {
	param.Set(value)
	return param
}

// String implements Param
func (param *ParamStrSlice) String() string {
	var ret strings.Builder
	ret.WriteString(param.library)
	ret.WriteString(".")
	ret.WriteString(param.name)
	ret.WriteString("=[ ")

	for _, v := range param.values {
		ret.WriteString("\"")
		ret.WriteString(v)
		ret.WriteString("\" ")
	}

	ret.WriteString("]")

	return ret.String()
}

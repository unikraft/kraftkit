// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ukargparse

import "fmt"

type Param interface {
	// The canonical name of the parameter which is understood by Unikraft.
	Name() string

	// Set the value of the parameter.
	Set(any)

	// Get the value of the parameter.
	Value() any

	// String returns the fully qualified parameter ready to be accepted by
	// Unikraft.
	String() string

	// A method-chain mechanism for both setting and getting the Param with the
	// newly embedded value.
	WithValue(any) Param
}

type paramStr struct {
	library string
	name    string
	value   string
}

// ParamStr instantiates a new Param based on a string value.
func ParamStr(lib string, name string, value any) Param {
	param := paramStr{
		library: lib,
		name:    name,
	}

	v, ok := value.(*string)
	if !ok {
		return &param
	}

	if v != nil {
		param.value = *v
	}

	return &param
}

// Name implements Param
func (param *paramStr) Name() string {
	return fmt.Sprintf("%s.%s", param.library, param.name)
}

// Set implements Param
func (param *paramStr) Set(value any) {
	v, ok := value.(string)
	if !ok {
		v, ok := value.(fmt.Stringer)
		if !ok {
			return
		}

		param.value = v.String()
		return
	}
	param.value = v
}

// Value implements Param
func (param *paramStr) Value() any {
	return param.value
}

// WithValue implements Param
func (param *paramStr) WithValue(value any) Param {
	param.Set(value)
	return param
}

// String implements Param
func (param *paramStr) String() string {
	return fmt.Sprintf("%s.%s=%s", param.library, param.name, param.value)
}

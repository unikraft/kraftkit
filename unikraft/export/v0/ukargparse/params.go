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
	Set(string)

	// Get the value of the parameter.
	Value() string

	// String returns the fully qualified parameter ready to be accepted by
	// Unikraft.
	String() string

	// A method-chain mechanism for both setting and getting the Param with the
	// newly embedded value.
	WithValue(string) Param
}

type paramStr struct {
	library string
	name    string
	value   string
}

// ParamStr instantiates a new Param based on a string value.
func ParamStr(lib string, name string, value *string) Param {
	param := paramStr{
		library: lib,
		name:    name,
	}

	if value != nil {
		param.value = *value
	}

	return &param
}

// Name implements Param
func (param *paramStr) Name() string {
	return fmt.Sprintf("%s.%s", param.library, param.name)
}

// Set implements Param
func (param *paramStr) Set(value string) {
	param.value = value
}

// Value implements Param
func (param *paramStr) Value() string {
	return fmt.Sprintf("%s.%s=%s", param.library, param.name, param.value)
}

// WithValue implements Param
func (param *paramStr) WithValue(value string) Param {
	param.value = value
	return param
}

// String implements Param
func (param *paramStr) String() string {
	return fmt.Sprintf("%s.%s=%s", param.library, param.name, param.value)
}

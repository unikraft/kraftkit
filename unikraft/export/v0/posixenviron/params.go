// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package posixenviron

import (
	"strings"

	"kraftkit.sh/unikraft/export/v0/ukargparse"
)

var ParamEnvVars = ukargparse.NewParamStrSlice("env", "vars", nil)

// ExportedParams returns the parameters available by this exported library.
func ExportedParams() []ukargparse.Param {
	return []ukargparse.Param{
		ParamEnvVars,
	}
}

// EnvVarEntry is an environment variable entry.
type EnvVarEntry struct {
	name  string
	value string
}

// NewEnvVarEntry generates a structure that is representative of one of
// Unikraft's posix-environ variables.
func NewEnvVarEntry(name, value string) EnvVarEntry {
	return EnvVarEntry{
		name,
		value,
	}
}

// String implements fmt.Stringer and returns a valid posix-environ environment
// variable.
func (entry EnvVarEntry) String() string {
	return strings.Join([]string{
		entry.name,
		entry.value,
	}, "=")
}

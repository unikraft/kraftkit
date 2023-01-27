// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2012 Alex Ogier.
// Copyright (c) 2012 The Go Authors.
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package cmdfactory

import (
	"github.com/spf13/pflag"
)

// VarF returns and instantiated flag based on a pointer value, a name and
// usage line.
func VarF(value pflag.Value, name, usage string) *pflag.Flag {
	flag := &pflag.Flag{
		Name:     name,
		Usage:    usage,
		Value:    value,
		DefValue: value.String(),
	}

	return flag
}

// VarPF is like VarP, but returns the flag created
func VarPF(value pflag.Value, name, shorthand, usage string) *pflag.Flag {
	flag := &pflag.Flag{
		Name:      name,
		Shorthand: shorthand,
		Usage:     usage,
		Value:     value,
		DefValue:  value.String(),
	}

	return flag
}

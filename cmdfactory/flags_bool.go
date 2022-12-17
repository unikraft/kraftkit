// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2012 Alex Ogier.
// Copyright (c) 2012 The Go Authors.
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package cmdfactory

import (
	"strconv"

	"github.com/spf13/pflag"
)

type boolValue bool

func newBoolValue(val bool, p *bool) *boolValue {
	*p = val
	return (*boolValue)(p)
}

func (b *boolValue) Set(val string) error {
	v, err := strconv.ParseBool(val)
	*b = boolValue(v)
	return err
}

func (b *boolValue) Type() string {
	return "bool"
}

func (b *boolValue) String() string { return strconv.FormatBool(bool(*b)) }

// BoolVarP is like BoolVar, but accepts a shorthand letter that can be used
// after a single dash.
func BoolVarP(p *bool, name, shorthand string, value bool, usage string) *pflag.Flag {
	flag := VarPF(newBoolValue(value, p), name, shorthand, usage)
	flag.NoOptDefVal = "true"
	return flag
}

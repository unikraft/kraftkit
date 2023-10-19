// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2012 Alex Ogier.
// Copyright (c) 2012 The Go Authors.
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package cmdfactory

import (
	"strconv"

	"github.com/spf13/pflag"
)

type intValue int

func newIntValue(val int, p *int) *intValue {
	*p = val
	return (*intValue)(p)
}

func (s *intValue) Set(val string) error {
	i, err := strconv.Atoi(val)
	if err != nil {
		return err
	}
	*s = intValue(i)
	return nil
}

func (s *intValue) Type() string {
	return "int"
}

func (s *intValue) String() string {
	return strconv.Itoa(int(*s))
}

// IntVar returns an instantiated flag for to an associated pointer string
// value with a given name, default value and usage line.
func IntVar(p *int, name string, value int, usage string) *pflag.Flag {
	return VarF(newIntValue(value, p), name, usage)
}

// IntVarP is like IntVar, but accepts a shorthand letter that can be used
// after a single dash.
func IntVarP(p *int, name, shorthand string, value int, usage string) *pflag.Flag {
	return VarPF(newIntValue(value, p), name, shorthand, usage)
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2012 Alex Ogier.
// Copyright (c) 2012 The Go Authors.
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package cmdfactory

import (
	"bytes"
	"encoding/csv"
	"strings"

	"github.com/spf13/pflag"
)

// -- stringArray Value
type stringArrayValue struct {
	value   *[]string
	changed bool
}

func newStringArrayValue(val []string, p *[]string) *stringArrayValue {
	ssv := new(stringArrayValue)
	ssv.value = p
	*ssv.value = val
	return ssv
}

func (s *stringArrayValue) Set(val string) error {
	if !s.changed {
		*s.value = []string{val}
		s.changed = true
	} else {
		*s.value = append(*s.value, val)
	}
	return nil
}

func (s *stringArrayValue) Append(val string) error {
	*s.value = append(*s.value, val)
	return nil
}

func (s *stringArrayValue) Replace(val []string) error {
	out := make([]string, len(val))
	for i, d := range val {
		var err error
		out[i] = d
		if err != nil {
			return err
		}
	}
	*s.value = out
	return nil
}

func (s *stringArrayValue) GetSlice() []string {
	out := make([]string, len(*s.value))
	copy(out, *s.value)
	return out
}

func (s *stringArrayValue) Type() string {
	return "strings"
}

func (s *stringArrayValue) String() string {
	str, _ := writeAsCSV(*s.value)
	return "[" + str + "]"
}

func writeAsCSV(vals []string) (string, error) {
	b := &bytes.Buffer{}
	w := csv.NewWriter(b)
	err := w.Write(vals)
	if err != nil {
		return "", err
	}
	w.Flush()
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// StringArrayVar defines a string flag with specified name, default value, and usage string.
// The argument p points to a []string variable in which to store the value of the flag.
// The value of each argument will not try to be separated by comma. Use a StringSlice for that.
func StringArrayVar(p *[]string, name string, value []string, usage string) *pflag.Flag {
	return VarF(newStringArrayValue(value, p), name, usage)
}

// StringArrayVarP is like StringArrayVar, but accepts a shorthand letter that can be used after a single dash.
func StringArrayVarP(p *[]string, name, shorthand string, value []string, usage string) *pflag.Flag {
	return VarPF(newStringArrayValue(value, p), name, shorthand, usage)
}

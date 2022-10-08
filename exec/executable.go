// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package exec

import (
	"fmt"
	"reflect"
	"strings"
)

type Executable struct {
	bin  string
	args []string
}

// NewExecutable accepts an input argument bin which is the path or executable
// name to be ultimately executed.  An optional positional argument args can be
// provided which is of an interface type.  The interface can use the attribute
// annotation tags `flag:"--myarg"` to aid serialization and organization of the
// executable's command-line arguments.  The type of the attribute will derive
// what is passed to the flag.
func NewExecutable(bin string, face interface{}, args ...string) (*Executable, error) {
	if len(bin) == 0 {
		return nil, fmt.Errorf("binary argument cannot be empty")
	}

	e := &Executable{}

	if strings.Contains(bin, " ") {
		args := strings.Split(bin, " ")
		bin = args[0]
		e.args = args[1:]
	}

	e.args = append(e.args, args...)
	e.bin = bin

	if face != nil {
		ifaceArgs, err := ParseInterfaceArgs(face)
		if err != nil {
			return nil, err
		}

		e.args = append(e.args, ifaceArgs...)
	}

	return e, nil
}

func (e *Executable) Args() []string {
	return e.args
}

type flag struct {
	flag        string
	omitvalueif string
}

func parseFlag(tag reflect.StructTag) (*flag, error) {
	parts := strings.Split(tag.Get("flag"), ",")
	if len(parts) == 0 {
		return nil, fmt.Errorf("could not parse flag without tag")
	}

	f := &flag{
		flag: parts[0],
	}

	for _, part := range parts[1:] {
		switch true {
		case strings.HasPrefix(part, "omitvalueif"):
			omit := strings.Split(part, "=")
			if len(omit) == 1 {
				return nil, fmt.Errorf("omitvalueif requires value")
			}
			f.omitvalueif = omit[1]

		default:
			continue
		}
	}

	return f, nil
}

// ParseInterfaceArgs returns the array of arguments detected from an interface
// with tag annotations `flag`
func ParseInterfaceArgs(face interface{}, args ...string) ([]string, error) {
	if face != nil && reflect.ValueOf(face).Kind() == reflect.Ptr {
		return nil, fmt.Errorf("cannot derive interface arguments from pointer: passed by reference")
	}

	t := reflect.TypeOf(face)
	v := reflect.ValueOf(face)

	for i := 0; i < t.NumField(); i++ {
		f, err := parseFlag(t.Field(i).Tag)
		if err != nil {
			continue
		}

		if len(f.flag) > 0 {
			switch v.Field(i).Type().String() {
			case "*int":
				if v.Field(i).IsZero() { // if nil
					continue
				}

				value := fmt.Sprintf("%d", reflect.Indirect(v.Field(i)).Int())

				if value == f.omitvalueif {
					args = append(args, f.flag)
				} else {
					args = append(args, f.flag)
					args = append(args, value)
				}

			case "bool":
				value := v.Field(i).Bool()
				if !value {
					continue
				}

				args = append(args, f.flag)

			case "[]string":
				n := v.Field(i).Len()
				if n == 0 {
					continue
				}

				for j := 0; j < n; j++ {
					args = append(args, f.flag)
					args = append(args, v.Field(i).Index(j).String())
				}

			case "string":
				value := v.Field(i).String()
				if len(value) == 0 {
					continue
				}

				args = append(args, f.flag)
				args = append(args, value)
			}

			// Recurisvely iterate through embedded structures
		} else if v.Field(i).Kind() == reflect.Struct {
			structArgs, err := ParseInterfaceArgs(v.Field(i).Interface())
			if err != nil {
				return []string{}, err
			}

			if len(structArgs) > 0 {
				args = append(args, structArgs...)
			}
		}
	}

	return args, nil
}

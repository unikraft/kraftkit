// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

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
			switch v.Field(i).Kind().String() {
			case "ptr":
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

			case "slice": // array of structs or custom slice type
				n := v.Field(i).Len()
				if n == 0 {
					continue
				}

				for j := 0; j < n; j++ {
					value, ok := v.Field(i).Index(j).Interface().(fmt.Stringer)
					if !ok {
						continue
					}

					str := value.String()
					if len(str) == 0 {
						continue
					}

					args = append(args, f.flag)
					args = append(args, str)
				}

			default:
				if !v.Field(i).CanInterface() {
					continue
				}

				value, ok := v.Field(i).Interface().(fmt.Stringer)
				if !ok {
					continue
				}

				str := value.String()
				if len(str) == 0 {
					continue
				}

				args = append(args, f.flag)
				args = append(args, str)
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

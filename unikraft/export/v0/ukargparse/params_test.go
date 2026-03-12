// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ukargparse

import (
	"testing"
)

func TestParamStr_Name(t *testing.T) {
	tests := []struct {
		name    string
		lib     string
		param   string
		want    string
	}{
		{
			name:  "simple name",
			lib:   "mylib",
			param: "myparam",
			want:  "mylib.myparam",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := ParamStr(tc.lib, tc.param, nil)
			if got := p.Name(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParamStr_Value(t *testing.T) {
	tests := []struct {
		name  string
		value *string
		want  string
	}{
		{
			name:  "nil value",
			value: nil,
			want:  "",
		},
		{
			name:  "non-empty value",
			value: func() *string { s := "hello"; return &s }(),
			want:  "hello",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := ParamStr("lib", "param", tc.value)
			if got := p.Value().(string); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParamStr_Set(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "string value",
			value: "world",
			want:  "world",
		},
		{
			name:  "non-string value ignored",
			value: 42,
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := ParamStr("lib", "param", nil)
			p.Set(tc.value)
			if got := p.Value().(string); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParamStr_Set_Stringer(t *testing.T) {
	p := ParamStr("lib", "param", nil)
	s := "stringer-value"
	str := ParamStr("lib", "x", &s)
	p.Set(str)
	if got := p.Value().(string); got != "lib.x=stringer-value" {
		t.Errorf("got %q, want %q", got, "lib.x=stringer-value")
	}
}

func TestParamStr_WithValue(t *testing.T) {
	p := ParamStr("lib", "param", nil)
	p2 := p.WithValue("newval")
	if got := p2.Value().(string); got != "newval" {
		t.Errorf("got %q, want %q", got, "newval")
	}
}

func TestParamStr_String(t *testing.T) {
	tests := []struct {
		name  string
		lib   string
		param string
		value string
		want  string
	}{
		{
			name:  "formatted string",
			lib:   "mylib",
			param: "myparam",
			value: "myvalue",
			want:  "mylib.myparam=myvalue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := ParamStr(tc.lib, tc.param, &tc.value)
			if got := p.String(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

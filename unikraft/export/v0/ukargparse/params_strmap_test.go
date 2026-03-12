// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ukargparse

import (
	"testing"
)

func TestParamStrMap_Name(t *testing.T) {
	p := ParamStrMap("mylib", "mymap", map[string]string{})
	if got := p.Name(); got != "mylib.mymap" {
		t.Errorf("got %q, want %q", got, "mylib.mymap")
	}
}

func TestParamStrMap_Value(t *testing.T) {
	values := map[string]string{"key": "val"}
	p := ParamStrMap("lib", "map", values)
	got, ok := p.Value().(map[string]string)
	if !ok {
		t.Fatal("value is not map[string]string")
	}
	if got["key"] != "val" {
		t.Errorf("got %q, want %q", got["key"], "val")
	}
}

func TestParamStrMap_Set(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  map[string]string
	}{
		{
			name:  "valid map",
			value: map[string]string{"a": "b"},
			want:  map[string]string{"a": "b"},
		},
		{
			name:  "invalid type ignored",
			value: "not-a-map",
			want:  map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := ParamStrMap("lib", "map", map[string]string{})
			p.Set(tc.value)
			got, ok := p.Value().(map[string]string)
			if !ok {
				t.Fatal("value is not map[string]string")
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("key %q: got %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestParamStrMap_WithValue(t *testing.T) {
	p := ParamStrMap("lib", "map", map[string]string{})
	p2 := p.WithValue(map[string]string{"x": "y"})
	got, ok := p2.Value().(map[string]string)
	if !ok {
		t.Fatal("value is not map[string]string")
	}
	if got["x"] != "y" {
		t.Errorf("got %q, want %q", got["x"], "y")
	}
}

func TestParamStrMap_String(t *testing.T) {
	p := ParamStrMap("mylib", "mymap", map[string]string{"k": "v"})
	got := p.String()
	want := "mylib.mymap[k=v]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParamStrMap_String_Empty(t *testing.T) {
	p := ParamStrMap("lib", "map", map[string]string{})
	got := p.String()
	want := "lib.map[]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

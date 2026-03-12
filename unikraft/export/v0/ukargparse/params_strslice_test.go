// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ukargparse

import (
	"testing"
)

func TestNewParamStrSlice_Name(t *testing.T) {
	p := NewParamStrSlice("mylib", "myslice", nil)
	if got := p.Name(); got != "mylib.myslice" {
		t.Errorf("got %q, want %q", got, "mylib.myslice")
	}
}

func TestNewParamStrSlice_NilValues(t *testing.T) {
	p := NewParamStrSlice("lib", "slice", nil)
	got, ok := p.Value().([]string)
	if !ok {
		t.Fatal("value is not []string")
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestNewParamStrSlice_WithInitialValues(t *testing.T) {
	p := NewParamStrSlice("lib", "slice", []string{"a", "b"})
	got, ok := p.Value().([]string)
	if !ok {
		t.Fatal("value is not []string")
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("unexpected values: %v", got)
	}
}

func TestParamStrSlice_Set(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  []string
	}{
		{
			name:  "valid slice",
			value: []string{"x", "y"},
			want:  []string{"x", "y"},
		},
		{
			name:  "invalid type ignored",
			value: "not-a-slice",
			want:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParamStrSlice("lib", "slice", nil)
			p.Set(tc.value)
			got, ok := p.Value().([]string)
			if tc.want == nil {
				if ok && len(got) != 0 {
					t.Errorf("expected nil/empty, got %v", got)
				}
				return
			}
			if !ok {
				t.Fatal("value is not []string")
			}
			if len(got) != len(tc.want) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tc.want))
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParamStrSlice_WithValue(t *testing.T) {
	p := NewParamStrSlice("lib", "slice", nil)
	p2 := p.WithValue([]string{"hello"})
	got, ok := p2.Value().([]string)
	if !ok {
		t.Fatal("value is not []string")
	}
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("unexpected values: %v", got)
	}
}

func TestParamStrSlice_String(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{
			name:   "empty",
			values: nil,
			want:   "lib.slice=[ ]",
		},
		{
			name:   "single value",
			values: []string{"hello"},
			want:   `lib.slice=[ "hello" ]`,
		},
		{
			name:   "multiple values",
			values: []string{"a", "b"},
			want:   `lib.slice=[ "a" "b" ]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParamStrSlice("lib", "slice", tc.values)
			if got := p.String(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

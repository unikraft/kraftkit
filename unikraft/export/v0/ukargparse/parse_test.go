// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ukargparse

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		wantLen int
		wantStr []string
	}{
		{
			name:    "empty args",
			args:    []string{},
			wantErr: false,
			wantLen: 0,
		},
		{
			name:    "single valid arg",
			args:    []string{"lib.param=value"},
			wantErr: false,
			wantLen: 1,
			wantStr: []string{"lib.param=value"},
		},
		{
			name:    "multiple valid args",
			args:    []string{"lib.param=value", "lib2.param2=value2"},
			wantErr: false,
			wantLen: 2,
			wantStr: []string{"lib.param=value", "lib2.param2=value2"},
		},
		{
			name:    "missing equals",
			args:    []string{"lib.param"},
			wantErr: true,
		},
		{
			name:    "missing dot",
			args:    []string{"libparam=value"},
			wantErr: true,
		},
		{
			name:    "value with equals sign",
			args:    []string{"lib.param=val=ue"},
			wantErr: false,
			wantLen: 1,
			wantStr: []string{"lib.param=val=ue"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			params, err := Parse(tc.args...)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(params) != tc.wantLen {
				t.Fatalf("length: got %d, want %d", len(params), tc.wantLen)
			}
			for i, want := range tc.wantStr {
				if got := params[i].String(); got != want {
					t.Errorf("index %d: got %q, want %q", i, got, want)
				}
			}
		})
	}
}

func TestParams_Strings(t *testing.T) {
	params, err := Parse("lib.a=1", "lib.b=2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := params.Strings()
	want := []string{"lib.a=1", "lib.b=2"}
	if len(got) != len(want) {
		t.Fatalf("length: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParams_Contains(t *testing.T) {
	params, err := Parse("lib.a=1", "lib.b=2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val := "1"
	needle := ParamStr("lib", "a", &val)
	if !params.Contains(needle) {
		t.Error("expected Contains to return true for existing param")
	}

	val2 := "x"
	missing := ParamStr("lib", "z", &val2)
	if params.Contains(missing) {
		t.Error("expected Contains to return false for missing param")
	}
}

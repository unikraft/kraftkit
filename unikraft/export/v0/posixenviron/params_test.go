// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package posixenviron

import (
	"testing"
)

func TestExportedParams(t *testing.T) {
	params := ExportedParams()
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
	if got := params[0].Name(); got != "env.vars" {
		t.Errorf("got %q, want %q", got, "env.vars")
	}
}

func TestNewEnvVarEntry_String(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{
			name:  "simple entry",
			key:   "HOME",
			value: "/root",
			want:  "HOME=/root",
		},
		{
			name:  "empty value",
			key:   "EMPTY",
			value: "",
			want:  "EMPTY=",
		},
		{
			name:  "empty key",
			key:   "",
			value: "val",
			want:  "=val",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := NewEnvVarEntry(tc.key, tc.value)
			if got := entry.String(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

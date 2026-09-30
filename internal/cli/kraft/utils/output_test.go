// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import "testing"

func TestIsValidOutputFormat(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   bool
	}{
		{name: "default", format: "", want: true},
		{name: "json", format: "json", want: true},
		{name: "table", format: "table", want: true},
		{name: "yaml", format: "yaml", want: true},
		{name: "list", format: "list", want: true},
		{name: "raw", format: "raw", want: true},
		{name: "unknown", format: "xml", want: false},
		{name: "wrong case", format: "JSON", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidOutputFormat(tt.format); got != tt.want {
				t.Fatalf("IsValidOutputFormat(%q) = %v, want %v", tt.format, got, tt.want)
			}
		})
	}
}

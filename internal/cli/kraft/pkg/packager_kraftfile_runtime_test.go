// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pkg

import "testing"

func TestFormatRuntimeReference(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		version string
		want    string
	}{
		{name: "plain name", input: "base", version: "", want: "base"},
		{name: "plain name with version", input: "base", version: "latest", want: "base:latest"},
		{name: "full ref with embedded tag", input: "index.unikraft.io/acme/base-compat:acme", version: "", want: "index.unikraft.io/acme/base-compat:acme"},
		{name: "full ref with explicit version", input: "index.unikraft.io/acme/base-compat", version: "acme", want: "index.unikraft.io/acme/base-compat:acme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatRuntimeReference(tt.input, tt.version); got != tt.want {
				t.Fatalf("formatRuntimeReference(%q, %q) = %q, want %q", tt.input, tt.version, got, tt.want)
			}
		})
	}
}

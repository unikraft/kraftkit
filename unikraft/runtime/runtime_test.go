// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package runtime

import (
	"context"
	"testing"
)

func TestRuntime_SetName(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantName    string
		wantVersion string
		wantQuery   string
		wantRef     string
	}{
		{
			name:        "shorthand tag is split",
			input:       "base:latest",
			wantName:    "base",
			wantVersion: "latest",
			wantQuery:   "base",
			wantRef:     "base:latest",
		},
		{
			name:        "namespaced shorthand tag is split",
			input:       "team/base:stable",
			wantName:    "team/base",
			wantVersion: "stable",
			wantQuery:   "team/base",
			wantRef:     "team/base:stable",
		},
		{
			name:        "full registry ref stays atomic",
			input:       "index.unikraft.io/official/base:latest",
			wantName:    "index.unikraft.io/official/base:latest",
			wantVersion: "",
			wantQuery:   "index.unikraft.io/official/base:latest",
			wantRef:     "index.unikraft.io/official/base:latest",
		},
		{
			name:        "registry ref with port stays atomic",
			input:       "localhost:5000/acme/base:latest",
			wantName:    "localhost:5000/acme/base:latest",
			wantVersion: "",
			wantQuery:   "localhost:5000/acme/base:latest",
			wantRef:     "localhost:5000/acme/base:latest",
		},
		{
			name:        "digest ref stays atomic",
			input:       "ghcr.io/acme/base@sha256:deadbeef",
			wantName:    "ghcr.io/acme/base@sha256:deadbeef",
			wantVersion: "",
			wantQuery:   "ghcr.io/acme/base@sha256:deadbeef",
			wantRef:     "ghcr.io/acme/base@sha256:deadbeef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := &Runtime{}
			rt.SetName(tt.input)

			if rt.Name() != tt.wantName {
				t.Errorf("Name() = %q, want %q", rt.Name(), tt.wantName)
			}

			if rt.Version() != tt.wantVersion {
				t.Errorf("Version() = %q, want %q", rt.Version(), tt.wantVersion)
			}

			if rt.QueryName() != tt.wantQuery {
				t.Errorf("QueryName() = %q, want %q", rt.QueryName(), tt.wantQuery)
			}

			if rt.Reference() != tt.wantRef {
				t.Errorf("Reference() = %q, want %q", rt.Reference(), tt.wantRef)
			}
		})
	}
}

func TestTransformFromSchema_OCIReference(t *testing.T) {
	got, err := TransformFromSchema(context.Background(), "oci://index.unikraft.io/official/base:latest")
	if err != nil {
		t.Fatalf("TransformFromSchema() error = %v", err)
	}

	rt := got.(Runtime)
	if rt.Name() != "index.unikraft.io/official/base:latest" {
		t.Errorf("Name() = %q, want full OCI ref", rt.Name())
	}

	if rt.Version() != "" {
		t.Errorf("Version() = %q, want empty", rt.Version())
	}

	if rt.Source() != "index.unikraft.io/official/base:latest" {
		t.Errorf("Source() = %q, want full OCI ref", rt.Source())
	}
}

func TestHasExplicitTag(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "plain name", input: "base", want: false},
		{name: "shorthand tag", input: "base:latest", want: true},
		{name: "namespaced tag", input: "team/base:stable", want: true},
		{name: "registry ref without tag", input: "index.unikraft.io/official/base", want: false},
		{name: "registry ref with tag", input: "index.unikraft.io/official/base:latest", want: true},
		{name: "registry ref with path tag", input: "index.unikraft.io/acme/base-compat:acme", want: true},
		{name: "registry ref with port and tag", input: "localhost:5000/acme/base:latest", want: true},
		{name: "digest ref", input: "ghcr.io/acme/base@sha256:deadbeef", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasExplicitTag(tt.input); got != tt.want {
				t.Fatalf("HasExplicitTag(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

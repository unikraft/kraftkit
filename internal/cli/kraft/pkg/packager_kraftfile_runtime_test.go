// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pkg

import (
	"testing"

	ukruntime "kraftkit.sh/unikraft/runtime"
)

func runtimeFromReference(reference string) *ukruntime.Runtime {
	runtime := &ukruntime.Runtime{}
	runtime.SetName(reference)
	return runtime
}

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
		{name: "digest ref", input: "ghcr.io/acme/base@sha256:deadbeef", version: "", want: "ghcr.io/acme/base@sha256:deadbeef"},
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

func TestResolveRuntimeNameVersion(t *testing.T) {
	tests := []struct {
		name           string
		runtimeFlag    string
		projectRuntime *ukruntime.Runtime
		nameFlag       string
		wantName       string
		wantVersion    string
		wantErr        string
	}{
		{
			name:           "runtime tag overrides Kraftfile version",
			runtimeFlag:    "base:v2",
			projectRuntime: runtimeFromReference("base:v1"),
			wantName:       "base",
			wantVersion:    "v2",
		},
		{
			name:           "untagged runtime overrides Kraftfile version",
			runtimeFlag:    "base",
			projectRuntime: runtimeFromReference("other:v1"),
			wantName:       "base",
			wantVersion:    "latest",
		},
		{
			name:           "runtime with registry port and tag",
			runtimeFlag:    "localhost:5000/acme/base:v2",
			projectRuntime: runtimeFromReference("other:v1"),
			wantName:       "localhost:5000/acme/base:v2",
		},
		{
			name:           "runtime digest overrides Kraftfile version",
			runtimeFlag:    "ghcr.io/acme/base@sha256:deadbeef",
			projectRuntime: runtimeFromReference("other:v1"),
			wantName:       "ghcr.io/acme/base@sha256:deadbeef",
		},
		{
			name:        "untagged runtime defaults to latest",
			runtimeFlag: "base",
			wantName:    "base",
			wantVersion: "latest",
		},
		{
			name:           "Kraftfile runtime",
			projectRuntime: runtimeFromReference("base:v1"),
			wantName:       "base",
			wantVersion:    "v1",
		},
		{
			name:        "name flag defaults to latest",
			nameFlag:    "base",
			wantName:    "base",
			wantVersion: "latest",
		},
		{
			name:    "missing runtime name",
			wantErr: "no runtime name specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, version, err := resolveRuntimeNameVersion(tt.runtimeFlag, tt.projectRuntime, tt.nameFlag)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("resolveRuntimeNameVersion() error = %v, want %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveRuntimeNameVersion() error = %v", err)
			}
			if name != tt.wantName || version != tt.wantVersion {
				t.Fatalf("resolveRuntimeNameVersion() = (%q, %q), want (%q, %q)", name, version, tt.wantName, tt.wantVersion)
			}
		})
	}
}

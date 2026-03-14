// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package component

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
)

// mockComponent is a minimal implementation of Component for testing.
type mockComponent struct {
	name    string
	version string
}

func (m *mockComponent) Type() unikraft.ComponentType      { return unikraft.ComponentTypeLib }
func (m *mockComponent) Name() string                      { return m.name }
func (m *mockComponent) Version() string                   { return m.version }
func (m *mockComponent) String() string                    { return m.name + ":" + m.version }
func (m *mockComponent) Source() string                    { return "" }
func (m *mockComponent) Path() string                      { return "" }
func (m *mockComponent) KConfig() kconfig.KeyValueMap      { return nil }
func (m *mockComponent) PrintInfo(context.Context) string  { return "" }
func (m *mockComponent) MarshalYAML() (interface{}, error) { return nil, nil }
func (m *mockComponent) KConfigTree(_ context.Context, _ ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, nil
}

func TestNameAndVersion(t *testing.T) {
	tests := []struct {
		name string
		comp Component
		want string
	}{
		{
			name: "name and version formatted correctly",
			comp: &mockComponent{name: "musl", version: "stable"},
			want: "musl:stable",
		},
		{
			name: "empty version",
			comp: &mockComponent{name: "nginx", version: ""},
			want: "nginx:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NameAndVersion(tt.comp)
			if got != tt.want {
				t.Errorf("NameAndVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTranslateFromSchema(t *testing.T) {
	// Create a temp file and temp dir for file-path testing in parseStringProp.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "myfile")
	if err := os.WriteFile(tmpFile, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	tests := []struct {
		name        string
		input       interface{}
		wantErr     bool
		wantKeys    map[string]interface{} // expected keys; nil means just no-error check
		wantMissing []string               // keys that must NOT be present
	}{
		// --- string branch ---
		{
			name:     "string: plain value treated as version",
			input:    "stable",
			wantKeys: map[string]interface{}{"version": "stable"},
		},
		{
			name:     "string: URL sets source and extracts name",
			input:    "https://github.com/unikraft/lib-musl.git",
			wantKeys: map[string]interface{}{"source": "https://github.com/unikraft/lib-musl.git", "name": "musl"},
		},
		{
			name:     "string: URL with branch query param sets version",
			input:    "https://github.com/unikraft/lib-nginx?branch=stable",
			wantKeys: map[string]interface{}{"version": "stable"},
		},
		{
			name:     "string: URL with tag query param sets version",
			input:    "https://github.com/unikraft/lib-redis?tag=v1.0",
			wantKeys: map[string]interface{}{"version": "v1.0"},
		},
		{
			name:     "string: URL with version query param sets version",
			input:    "https://github.com/unikraft/lib-redis?version=v2.0",
			wantKeys: map[string]interface{}{"version": "v2.0"},
		},
		{
			name:        "string: URL with unrecognised query param has no version",
			input:       "https://github.com/unikraft/lib-redis?foo=bar",
			wantMissing: []string{"version"},
		},
		{
			name:     "string: URL with .tar.gz suffix stripped before name extraction",
			input:    "https://example.com/unikraft/lib-musl.tar.gz",
			wantKeys: map[string]interface{}{"name": "musl"},
		},
		{
			name:     "string: URL with single-segment path",
			input:    "https://example.com/lib-nginx",
			wantKeys: map[string]interface{}{"source": "https://example.com/lib-nginx", "name": "nginx"},
		},
		{
			name:        "string: URL where GuessTypeNameVersion fails leaves no name key",
			input:       "https://example.com/some.opaque.pkg",
			wantMissing: []string{"name"},
		},
		{
			name:     "string: existing file path sets source",
			input:    tmpFile,
			wantKeys: map[string]interface{}{"source": tmpFile},
		},
		{
			name:     "string: existing directory path sets source",
			input:    tmpDir,
			wantKeys: map[string]interface{}{"source": tmpDir},
		},

		// --- map branch: version ---
		{
			name:     "map: version as string",
			input:    map[string]interface{}{"version": "1.0"},
			wantKeys: map[string]interface{}{"version": "1.0"},
		},
		{
			name:     "map: version as int converted to string",
			input:    map[string]interface{}{"version": 42},
			wantKeys: map[string]interface{}{"version": "42"},
		},
		{
			name:     "map: version as float uses fmt.Sprint",
			input:    map[string]interface{}{"version": 3.14},
			wantKeys: map[string]interface{}{"version": "3.14"},
		},

		// --- map branch: source ---
		{
			name:     "map: source as valid string URL sets source",
			input:    map[string]interface{}{"source": "https://github.com/unikraft/lib-musl.git"},
			wantKeys: map[string]interface{}{"source": "https://github.com/unikraft/lib-musl.git"},
		},
		{
			name:    "map: source as non-string returns error",
			input:   map[string]interface{}{"source": 123},
			wantErr: true,
		},

		// --- map branch: kconfig ---
		{
			name: "map: kconfig as map[string]interface{}",
			input: map[string]interface{}{
				"kconfig": map[string]interface{}{"CONFIG_FOO": "y"},
			},
		},
		{
			name: "map: kconfig as []interface{}",
			input: map[string]interface{}{
				"kconfig": []interface{}{"CONFIG_BAR=y"},
			},
		},
		{
			name: "map: kconfig as slice with invalid entry returns error",
			input: map[string]interface{}{
				"kconfig": []interface{}{"INVALID_NO_EQUALS"},
			},
			wantErr: true,
		},
		{
			name: "map: kconfig as map with nil value returns error",
			input: map[string]interface{}{
				"kconfig": map[string]interface{}{"CONFIG_NIL": nil},
			},
			wantErr: true,
		},

		// --- map branch: default (unknown key pass-through) ---
		{
			name:     "map: unknown key is passed through",
			input:    map[string]interface{}{"custom_key": "custom_value"},
			wantKeys: map[string]interface{}{"custom_key": "custom_value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TranslateFromSchema(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TranslateFromSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			for k, wantVal := range tt.wantKeys {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("TranslateFromSchema() missing key %q", k)
					continue
				}
				if wantVal != nil {
					if gotVal != wantVal {
						t.Errorf("TranslateFromSchema()[%q] = %v (%T), want %v (%T)", k, gotVal, gotVal, wantVal, wantVal)
					}
				}
			}

			for _, k := range tt.wantMissing {
				if _, ok := got[k]; ok {
					t.Errorf("TranslateFromSchema() unexpected key %q present", k)
				}
			}
		})
	}
}

func TestUrlHasVersion(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "empty query returns empty string",
			query: "",
			want:  "",
		},
		{
			name:  "branch param returns its value",
			query: "branch=stable",
			want:  "stable",
		},
		{
			name:  "tag param returns its value",
			query: "tag=v1.0",
			want:  "v1.0",
		},
		{
			name:  "version param returns its value",
			query: "version=v2.0",
			want:  "v2.0",
		},
		{
			name:  "unrecognised param returns empty string",
			query: "foo=bar",
			want:  "",
		},
		{
			name:  "branch takes priority over tag",
			query: "branch=main&tag=v1",
			want:  "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &url.URL{RawQuery: tt.query}
			got := urlHasVersion(u)
			if got != tt.want {
				t.Errorf("urlHasVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseStringProp(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(tmpFile, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	tests := []struct {
		name        string
		entry       string
		wantKeys    map[string]interface{}
		wantMissing []string
	}{
		{
			name:     "existing regular file sets source",
			entry:    tmpFile,
			wantKeys: map[string]interface{}{"source": tmpFile},
		},
		{
			name:     "existing directory sets source",
			entry:    tmpDir,
			wantKeys: map[string]interface{}{"source": tmpDir},
		},
		{
			name:     "plain string is treated as version",
			entry:    "stable",
			wantKeys: map[string]interface{}{"version": "stable"},
		},
		{
			name:     "URL with host sets source",
			entry:    "https://github.com/unikraft/lib-musl",
			wantKeys: map[string]interface{}{"source": "https://github.com/unikraft/lib-musl"},
		},
		{
			name:     "URL with .git suffix strips it before name extraction",
			entry:    "https://github.com/unikraft/lib-musl.git",
			wantKeys: map[string]interface{}{"name": "musl"},
		},
		{
			name:     "URL with .tar.gz suffix strips it before name extraction",
			entry:    "https://example.com/unikraft/lib-nginx.tar.gz",
			wantKeys: map[string]interface{}{"name": "nginx"},
		},
		{
			name:     "URL with branch query sets version",
			entry:    "https://github.com/unikraft/lib-musl?branch=staging",
			wantKeys: map[string]interface{}{"version": "staging"},
		},
		{
			name:        "URL where name cannot be guessed has no name key",
			entry:       "https://example.com/some.opaque.pkg",
			wantMissing: []string{"name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStringProp(tt.entry)

			for k, wantVal := range tt.wantKeys {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("parseStringProp() missing key %q", k)
					continue
				}
				if gotVal != wantVal {
					t.Errorf("parseStringProp()[%q] = %v, want %v", k, gotVal, wantVal)
				}
			}

			for _, k := range tt.wantMissing {
				if _, ok := got[k]; ok {
					t.Errorf("parseStringProp() unexpected key %q present", k)
				}
			}
		})
	}
}

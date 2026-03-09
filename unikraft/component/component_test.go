// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package component

import (
	"context"
	"net/url"
	"os"
	"testing"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
)

// ---------------------------------------------------------------------------
// mock Component for testing NameAndVersion
// ---------------------------------------------------------------------------

type mockComponent struct {
	name    string
	version string
}

func (m *mockComponent) Name() string                    { return m.name }
func (m *mockComponent) Version() string                 { return m.version }
func (m *mockComponent) String() string                  { return m.name }
func (m *mockComponent) Type() unikraft.ComponentType    { return unikraft.ComponentTypeApp }
func (m *mockComponent) Source() string                  { return "" }
func (m *mockComponent) Path() string                    { return "" }
func (m *mockComponent) IsUnpacked() bool                { return false }
func (m *mockComponent) KConfig() kconfig.KeyValueMap    { return kconfig.KeyValueMap{} }
func (m *mockComponent) PrintInfo(_ context.Context) string { return "" }
func (m *mockComponent) MarshalYAML() (interface{}, error) { return nil, nil }
func (m *mockComponent) KConfigTree(_ context.Context, _ ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// NameAndVersion
// ---------------------------------------------------------------------------

func TestNameAndVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"myapp", "0.15.0", "myapp:0.15.0"},
		{"nginx", "stable", "nginx:stable"},
		{"app", "", "app:"},
		{"", "1.0", ":1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			c := &mockComponent{name: tt.name, version: tt.version}
			if got := NameAndVersion(c); got != tt.want {
				t.Errorf("NameAndVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// urlHasVersion
// ---------------------------------------------------------------------------

func TestUrlHasVersion(t *testing.T) {
	tests := []struct {
		name  string
		rawURL string
		want  string
	}{
		{
			name:   "branch query param",
			rawURL: "https://github.com/unikraft/unikraft?branch=stable",
			want:   "stable",
		},
		{
			name:   "tag query param",
			rawURL: "https://github.com/unikraft/unikraft?tag=v0.15.0",
			want:   "v0.15.0",
		},
		{
			name:   "version query param",
			rawURL: "https://github.com/unikraft/unikraft?version=0.15.0",
			want:   "0.15.0",
		},
		{
			name:   "no relevant query param",
			rawURL: "https://github.com/unikraft/unikraft?foo=bar",
			want:   "",
		},
		{
			name:   "empty query",
			rawURL: "https://github.com/unikraft/unikraft",
			want:   "",
		},
		{
			name:   "branch takes priority over tag",
			rawURL: "https://github.com/unikraft/unikraft?branch=main&tag=v1.0",
			want:   "main",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.rawURL)
			if err != nil {
				t.Fatalf("url.Parse(%q) error = %v", tt.rawURL, err)
			}
			if got := urlHasVersion(u); got != tt.want {
				t.Errorf("urlHasVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseStringProp
// ---------------------------------------------------------------------------

func TestParseStringProp(t *testing.T) {
	// Create a temp file and dir for local path tests
	tmpFile, err := os.CreateTemp("", "component-test-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tmpDir, err := os.MkdirTemp("", "component-test-dir-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name       string
		entry      string
		wantSource string
		wantVersion string
		wantName   string
	}{
		{
			name:       "existing file sets source",
			entry:      tmpFile.Name(),
			wantSource: tmpFile.Name(),
		},
		{
			name:       "existing directory sets source",
			entry:      tmpDir,
			wantSource: tmpDir,
		},
		{
			name:        "plain version string",
			entry:       "0.15.0",
			wantVersion: "0.15.0",
		},
		{
			name:        "stable string treated as version",
			entry:       "stable",
			wantVersion: "stable",
		},
		{
			name:       "github URL sets source",
			entry:      "https://github.com/unikraft/unikraft",
			wantSource: "https://github.com/unikraft/unikraft",
		},
		{
			name:       "github URL with .git extension",
			entry:      "https://github.com/unikraft/unikraft.git",
			wantSource: "https://github.com/unikraft/unikraft.git",
		},
		{
			name:        "URL with branch query param sets version",
			entry:       "https://github.com/unikraft/unikraft?branch=stable",
			wantSource:  "https://github.com/unikraft/unikraft?branch=stable",
			wantVersion: "stable",
		},
		{
			name:       "URL with single-segment path",
			entry:      "https://example.com/unikraft",
			wantSource: "https://example.com/unikraft",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStringProp(tt.entry)

			if tt.wantSource != "" {
				if got["source"] != tt.wantSource {
					t.Errorf("parseStringProp()[source] = %v, want %q", got["source"], tt.wantSource)
				}
			} else {
				if _, ok := got["source"]; ok {
					t.Errorf("parseStringProp() unexpected source key: %v", got["source"])
				}
			}

			if tt.wantVersion != "" {
				if got["version"] != tt.wantVersion {
					t.Errorf("parseStringProp()[version] = %v, want %q", got["version"], tt.wantVersion)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TranslateFromSchema
// ---------------------------------------------------------------------------

func TestTranslateFromSchema(t *testing.T) {
	t.Run("string input sets version", func(t *testing.T) {
		got, err := TranslateFromSchema("0.15.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["version"] != "0.15.0" {
			t.Errorf("version = %v, want %q", got["version"], "0.15.0")
		}
	})

	t.Run("map with version as string", func(t *testing.T) {
		got, err := TranslateFromSchema(map[string]interface{}{
			"version": "0.15.0",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["version"] != "0.15.0" {
			t.Errorf("version = %v, want %q", got["version"], "0.15.0")
		}
	})

	t.Run("map with version as int", func(t *testing.T) {
		got, err := TranslateFromSchema(map[string]interface{}{
			"version": 15,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["version"] != "15" {
			t.Errorf("version = %v, want %q", got["version"], "15")
		}
	})

	t.Run("map with version as other type", func(t *testing.T) {
		got, err := TranslateFromSchema(map[string]interface{}{
			"version": true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["version"] != "true" {
			t.Errorf("version = %v, want %q", got["version"], "true")
		}
	})

	t.Run("map with source as valid string", func(t *testing.T) {
		got, err := TranslateFromSchema(map[string]interface{}{
			"source": "0.15.0",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["version"] != "0.15.0" {
			t.Errorf("version = %v, want %q", got["version"], "0.15.0")
		}
	})

	t.Run("map with source as non-string errors", func(t *testing.T) {
		_, err := TranslateFromSchema(map[string]interface{}{
			"source": 123,
		})
		if err == nil {
			t.Errorf("expected error for non-string source, got nil")
		}
	})

	t.Run("map with kconfig as map", func(t *testing.T) {
		got, err := TranslateFromSchema(map[string]interface{}{
			"kconfig": map[string]interface{}{
				"CONFIG_TEST": "y",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["kconfig"] == nil {
			t.Errorf("kconfig should not be nil")
		}
	})

	t.Run("map with kconfig as slice", func(t *testing.T) {
		got, err := TranslateFromSchema(map[string]interface{}{
			"kconfig": []interface{}{"CONFIG_TEST=y"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["kconfig"] == nil {
			t.Errorf("kconfig should not be nil")
		}
	})

	t.Run("map with unknown key is passed through", func(t *testing.T) {
		got, err := TranslateFromSchema(map[string]interface{}{
			"custom_key": "custom_value",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got["custom_key"] != "custom_value" {
			t.Errorf("custom_key = %v, want %q", got["custom_key"], "custom_value")
		}
	})

	t.Run("unsupported input type returns empty map", func(t *testing.T) {
		got, err := TranslateFromSchema(12345)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty map, got %v", got)
		}
	})

	t.Run("map with kconfig slice missing value errors", func(t *testing.T) {
		_, err := TranslateFromSchema(map[string]interface{}{
			"kconfig": []interface{}{"CONFIG_TEST"},
		})
		if err == nil {
			t.Errorf("expected error for kconfig slice with key-only entry, got nil")
		}
	})

	t.Run("map with kconfig map with nil value errors", func(t *testing.T) {
		_, err := TranslateFromSchema(map[string]interface{}{
			"kconfig": map[string]interface{}{
				"CONFIG_TEST": nil,
			},
		})
		if err == nil {
			t.Errorf("expected error for kconfig map with nil value, got nil")
		}
	})
}

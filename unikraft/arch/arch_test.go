// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package arch

import (
	"context"
	"os"
	"runtime"
	"testing"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
)

// ---------------------------------------------------------------------------
// ArchitectureName
// ---------------------------------------------------------------------------

func TestArchitectureName_String(t *testing.T) {
	tests := []struct {
		name ArchitectureName
		want string
	}{
		{ArchitectureX86_64, "x86_64"},
		{ArchitectureArm64, "arm64"},
		{ArchitectureArm, "arm"},
		{ArchitectureUnknown, "unknown"},
	}
	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			if got := tt.name.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ArchitectureByName
// ---------------------------------------------------------------------------

func TestArchitectureByName(t *testing.T) {
	tests := []struct {
		input string
		want  ArchitectureName
	}{
		{"x86_64", ArchitectureX86_64},
		{"arm64", ArchitectureArm64},
		{"arm", ArchitectureArm},
		{"unknown_arch", ArchitectureUnknown},
		{"", ArchitectureUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ArchitectureByName(tt.input); got != tt.want {
				t.Errorf("ArchitectureByName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ArchitecturesByName
// ---------------------------------------------------------------------------

func TestArchitecturesByName(t *testing.T) {
	m := ArchitecturesByName()
	expected := map[string]ArchitectureName{
		"x86_64": ArchitectureX86_64,
		"arm64":  ArchitectureArm64,
		"arm":    ArchitectureArm,
	}
	if len(m) != len(expected) {
		t.Errorf("ArchitecturesByName() len = %d, want %d", len(m), len(expected))
	}
	for k, v := range expected {
		if m[k] != v {
			t.Errorf("ArchitecturesByName()[%q] = %q, want %q", k, m[k], v)
		}
	}
}

// ---------------------------------------------------------------------------
// Architectures
// ---------------------------------------------------------------------------

func TestArchitectures(t *testing.T) {
	archs := Architectures()
	if len(archs) != 3 {
		t.Errorf("Architectures() len = %d, want 3", len(archs))
	}
	found := map[ArchitectureName]bool{}
	for _, a := range archs {
		found[a] = true
	}
	for _, want := range []ArchitectureName{ArchitectureX86_64, ArchitectureArm64, ArchitectureArm} {
		if !found[want] {
			t.Errorf("Architectures() missing %q", want)
		}
	}
}

// ---------------------------------------------------------------------------
// ArchitectureAliases
// ---------------------------------------------------------------------------

func TestArchitectureAliases(t *testing.T) {
	aliases := ArchitectureAliases()
	// Each known arch must appear as a key
	for _, arch := range Architectures() {
		if _, ok := aliases[arch]; !ok {
			t.Errorf("ArchitectureAliases() missing key %q", arch)
		}
	}
	// Total alias count must equal total entries in ArchitecturesByName
	total := 0
	for _, v := range aliases {
		total += len(v)
	}
	if total != len(ArchitecturesByName()) {
		t.Errorf("ArchitectureAliases() total aliases = %d, want %d", total, len(ArchitecturesByName()))
	}
}

// ---------------------------------------------------------------------------
// NewArchitectureFromSchema
// ---------------------------------------------------------------------------

func TestNewArchitectureFromSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		wantName string
	}{
		{
			name:     "valid name",
			input:    "x86_64",
			wantErr:  false,
			wantName: "x86_64",
		},
		{
			name:     "custom name",
			input:    "arm64",
			wantErr:  false,
			wantName: "arm64",
		},
		{
			name:    "empty string errors",
			input:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewArchitectureFromSchema(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewArchitectureFromSchema(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Name() != tt.wantName {
				t.Errorf("NewArchitectureFromSchema(%q).Name() = %q, want %q", tt.input, got.Name(), tt.wantName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ArchitectureConfig accessors
// ---------------------------------------------------------------------------

func TestArchitectureConfig_Accessors(t *testing.T) {
	ac := ArchitectureConfig{
		name:    "x86_64",
		version: "0.15.0",
		source:  "https://github.com/unikraft/unikraft",
		path:    "/tmp/unikraft",
	}

	if got := ac.Name(); got != "x86_64" {
		t.Errorf("Name() = %q, want %q", got, "x86_64")
	}
	if got := ac.String(); got != "x86_64" {
		t.Errorf("String() = %q, want %q", got, "x86_64")
	}
	if got := ac.Version(); got != "0.15.0" {
		t.Errorf("Version() = %q, want %q", got, "0.15.0")
	}
	if got := ac.Source(); got != "https://github.com/unikraft/unikraft" {
		t.Errorf("Source() = %q, want %q", got, "https://github.com/unikraft/unikraft")
	}
	if got := ac.Path(); got != "/tmp/unikraft" {
		t.Errorf("Path() = %q, want %q", got, "/tmp/unikraft")
	}
	if got := ac.Type(); got != unikraft.ComponentTypeArch {
		t.Errorf("Type() = %v, want %v", got, unikraft.ComponentTypeArch)
	}
}

// ---------------------------------------------------------------------------
// IsUnpacked
// ---------------------------------------------------------------------------

func TestArchitectureConfig_IsUnpacked(t *testing.T) {
	t.Run("existing directory returns true", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "arch-test-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		ac := ArchitectureConfig{path: dir}
		if !ac.IsUnpacked() {
			t.Errorf("IsUnpacked() = false, want true for existing dir")
		}
	})

	t.Run("non-existent path returns false", func(t *testing.T) {
		ac := ArchitectureConfig{path: "/nonexistent/path/xyz"}
		if ac.IsUnpacked() {
			t.Errorf("IsUnpacked() = true, want false for non-existent path")
		}
	})

	t.Run("empty path returns false", func(t *testing.T) {
		ac := ArchitectureConfig{}
		if ac.IsUnpacked() {
			t.Errorf("IsUnpacked() = true, want false for empty path")
		}
	})
}

// ---------------------------------------------------------------------------
// KConfig
// ---------------------------------------------------------------------------

func TestArchitectureConfig_KConfig(t *testing.T) {
	tests := []struct {
		archName string
		wantKey  string
	}{
		{"x86_64", "CONFIG_ARCH_X86_64"},
		{"arm64", "CONFIG_ARCH_ARM_64"},
		{"arm", "CONFIG_ARCH_ARM_32"},
		{"unknown", "CONFIG_"},
	}
	for _, tt := range tests {
		t.Run(tt.archName, func(t *testing.T) {
			ac := ArchitectureConfig{name: tt.archName}
			kconf := ac.KConfig()
			if _, exists := kconf.Get(tt.wantKey); !exists {
				t.Errorf("KConfig() missing key %q for arch %q, got: %v", tt.wantKey, tt.archName, kconf)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MarshalYAML
// ---------------------------------------------------------------------------

func TestArchitectureConfig_MarshalYAML(t *testing.T) {
	ac := ArchitectureConfig{name: "x86_64"}
	got, err := ac.MarshalYAML()
	if err != nil {
		t.Errorf("MarshalYAML() unexpected error = %v", err)
	}
	if got != nil {
		t.Errorf("MarshalYAML() = %v, want nil", got)
	}
}

// ---------------------------------------------------------------------------
// PrintInfo
// ---------------------------------------------------------------------------

func TestArchitectureConfig_PrintInfo(t *testing.T) {
	ac := ArchitectureConfig{name: "x86_64"}
	got := ac.PrintInfo(context.Background())
	if got == "" {
		t.Errorf("PrintInfo() returned empty string")
	}
}

// ---------------------------------------------------------------------------
// KConfigTree
// ---------------------------------------------------------------------------

func TestArchitectureConfig_KConfigTree(t *testing.T) {
	ac := ArchitectureConfig{name: "x86_64"}
	tree, err := ac.KConfigTree(context.Background())
	if err != nil {
		t.Errorf("KConfigTree() unexpected error = %v", err)
	}
	if tree != nil {
		t.Errorf("KConfigTree() = %v, want nil", tree)
	}
}

// ---------------------------------------------------------------------------
// TransformFromSchema
// ---------------------------------------------------------------------------

func TestTransformFromSchema(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		input    interface{}
		wantName string
		wantErr  bool
	}{
		{
			name:     "string input sets name",
			ctx:      context.Background(),
			input:    "x86_64",
			wantName: "x86_64",
		},
		{
			name:     "arm64 string input",
			ctx:      context.Background(),
			input:    "arm64",
			wantName: "arm64",
		},
		{
			name:    "non-string input errors",
			ctx:     context.Background(),
			input:   123,
			wantErr: true,
		},
		{
			name:    "map input errors",
			ctx:     context.Background(),
			input:   map[string]interface{}{"name": "x86_64"},
			wantErr: true,
		},
		{
			name: "with UK_BASE context sets path",
			ctx: unikraft.WithContext(context.Background(), &unikraft.Context{
				UK_BASE: "/tmp/unikraft-base",
			}),
			input:    "x86_64",
			wantName: "x86_64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransformFromSchema(tt.ctx, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("TransformFromSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			ac, ok := got.(ArchitectureConfig)
			if !ok {
				t.Fatalf("TransformFromSchema() result type = %T, want ArchitectureConfig", got)
			}
			if ac.Name() != tt.wantName {
				t.Errorf("TransformFromSchema() Name() = %q, want %q", ac.Name(), tt.wantName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HostArchitecture
// ---------------------------------------------------------------------------

func TestHostArchitecture(t *testing.T) {
	got, err := HostArchitecture()

	switch runtime.GOARCH {
	case "amd64":
		if err != nil {
			t.Errorf("HostArchitecture() unexpected error = %v", err)
		}
		if got != "x86_64" {
			t.Errorf("HostArchitecture() = %q, want %q", got, "x86_64")
		}
	case "arm", "arm64":
		if err != nil {
			t.Errorf("HostArchitecture() unexpected error = %v", err)
		}
		if got != runtime.GOARCH {
			t.Errorf("HostArchitecture() = %q, want %q", got, runtime.GOARCH)
		}
	default:
		if err == nil {
			t.Errorf("HostArchitecture() expected error for unsupported arch %q, got nil", runtime.GOARCH)
		}
	}
}

// ---------------------------------------------------------------------------
// NewArchitectureFromOptions
// ---------------------------------------------------------------------------

func TestNewArchitectureFromOptions(t *testing.T) {
	arch := NewArchitectureFromOptions(
		WithName("x86_64"),
		WithVersion("0.15.0"),
		WithSource("https://github.com/unikraft/unikraft"),
		WithPath("/tmp/unikraft"),
	)

	if arch.Name() != "x86_64" {
		t.Errorf("Name() = %q, want %q", arch.Name(), "x86_64")
	}
	if arch.Version() != "0.15.0" {
		t.Errorf("Version() = %q, want %q", arch.Version(), "0.15.0")
	}
	if arch.Source() != "https://github.com/unikraft/unikraft" {
		t.Errorf("Source() = %q, want %q", arch.Source(), "https://github.com/unikraft/unikraft")
	}
	if arch.Path() != "/tmp/unikraft" {
		t.Errorf("Path() = %q, want %q", arch.Path(), "/tmp/unikraft")
	}
}

func TestWithKConfig(t *testing.T) {
	kv := kconfig.KeyValueMap{}
	kv.Set("CONFIG_ARCH_X86_64", kconfig.Yes)

	arch := NewArchitectureFromOptions(
		WithName("x86_64"),
		WithKConfig(kv),
	)

	ac := arch.(*ArchitectureConfig)
	got := ac.KConfig()
	if _, exists := got.Get("CONFIG_ARCH_X86_64"); !exists {
		t.Errorf("KConfig() missing CONFIG_ARCH_X86_64 after WithKConfig")
	}
}

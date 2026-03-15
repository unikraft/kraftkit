// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package plat

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
)

// --- TransformFromSchema tests ---

func TestTransformFromSchema(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		ctx      context.Context
		input    interface{}
		wantName string
		wantErr  bool
	}{
		{
			name:     "string input sets name",
			ctx:      ctx,
			input:    "kvm",
			wantName: "qemu", // "kvm" is an alias for "qemu"
		},
		{
			name:     "string input with non-alias name",
			ctx:      ctx,
			input:    "xen",
			wantName: "xen",
		},
		{
			name:     "string input with unknown platform name preserved",
			ctx:      ctx,
			input:    "myplatform",
			wantName: "myplatform",
		},
		{
			name:     "firecracker alias resolved to fc",
			ctx:      ctx,
			input:    "firecracker",
			wantName: "fc",
		},
		{
			name:    "integer input returns error",
			ctx:     ctx,
			input:   123,
			wantErr: true,
		},
		{
			name:    "map input returns error",
			ctx:     ctx,
			input:   map[string]interface{}{"name": "kvm"},
			wantErr: true,
		},
		{
			name:    "nil input returns error",
			ctx:     ctx,
			input:   nil,
			wantErr: true,
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

			pc, ok := got.(PlatformConfig)
			if !ok {
				t.Fatalf("TransformFromSchema() result type = %T, want PlatformConfig", got)
			}

			if tt.wantName != "" && pc.name != tt.wantName {
				t.Errorf("name = %q, want %q", pc.name, tt.wantName)
			}
		})
	}
}

func TestTransformFromSchema_withUKBase(t *testing.T) {
	ukBase := t.TempDir()
	ctx := unikraft.WithContext(context.Background(), &unikraft.Context{
		UK_BASE: ukBase,
	})

	got, err := TransformFromSchema(ctx, "qemu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pc := got.(PlatformConfig)
	if pc.name != "qemu" {
		t.Errorf("name = %q, want %q", pc.name, "qemu")
	}
	if pc.path == "" {
		t.Error("expected path to be set when UK_BASE is provided")
	}
	expectedPath := filepath.Join(ukBase, unikraft.VendorDir, unikraft.ComponentTypePlat.Plural(), "qemu")
	if pc.path != expectedPath {
		t.Errorf("path = %q, want %q", pc.path, expectedPath)
	}
}

func TestTransformFromSchema_nilUKContext(t *testing.T) {
	ctx := unikraft.WithContext(context.Background(), nil)

	got, err := TransformFromSchema(ctx, "xen")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pc := got.(PlatformConfig)
	if pc.name != "xen" {
		t.Errorf("name = %q, want %q", pc.name, "xen")
	}
	if pc.path != "" {
		t.Errorf("path should be empty without UK_BASE, got %q", pc.path)
	}
}

func TestTransformFromSchema_emptyUKBase(t *testing.T) {
	ctx := unikraft.WithContext(context.Background(), &unikraft.Context{
		UK_BASE: "",
	})

	got, err := TransformFromSchema(ctx, "linuxu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pc := got.(PlatformConfig)
	if pc.path != "" {
		t.Errorf("path should be empty when UK_BASE is empty, got %q", pc.path)
	}
}

// --- PlatformConfig method tests ---

func TestPlatformConfig_methods(t *testing.T) {
	kv, _ := kconfig.NewKeyValueMapFromMap(map[string]interface{}{"CONFIG_PLAT_KVM": "y"})
	pc := PlatformConfig{
		name:    "qemu",
		version: "0.14.0",
		source:  "https://example.com/qemu",
		path:    "/some/path",
		kconfig: kv,
	}

	if pc.Name() != "qemu" {
		t.Errorf("Name() = %q, want %q", pc.Name(), "qemu")
	}
	if pc.String() != "qemu" {
		t.Errorf("String() = %q, want %q", pc.String(), "qemu")
	}
	if pc.Version() != "0.14.0" {
		t.Errorf("Version() = %q, want %q", pc.Version(), "0.14.0")
	}
	if pc.Source() != "https://example.com/qemu" {
		t.Errorf("Source() = %q, want %q", pc.Source(), "https://example.com/qemu")
	}
	if pc.Path() != "/some/path" {
		t.Errorf("Path() = %q, want %q", pc.Path(), "/some/path")
	}
	if pc.Type() != unikraft.ComponentTypePlat {
		t.Errorf("Type() = %v, want %v", pc.Type(), unikraft.ComponentTypePlat)
	}
	if pc.KConfig() == nil {
		t.Error("KConfig() should not be nil")
	}

	ctx := context.Background()
	kconf, err := pc.KConfigTree(ctx)
	if err != nil || kconf != nil {
		t.Errorf("KConfigTree() = %v, %v; want nil, nil", kconf, err)
	}

	info := pc.PrintInfo(ctx)
	if info == "" {
		t.Error("PrintInfo() should return non-empty string")
	}
}

func TestPlatformConfig_IsUnpacked(t *testing.T) {
	t.Run("existing directory returns true", func(t *testing.T) {
		tmpDir := t.TempDir()
		pc := PlatformConfig{path: tmpDir}
		if !pc.IsUnpacked() {
			t.Error("IsUnpacked() = false, want true for existing directory")
		}
	})

	t.Run("non-existing path returns false", func(t *testing.T) {
		pc := PlatformConfig{path: "/nonexistent/path/to/platform"}
		if pc.IsUnpacked() {
			t.Error("IsUnpacked() = true, want false for non-existing path")
		}
	})

	t.Run("file path returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "file.txt")
		if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		pc := PlatformConfig{path: tmpFile}
		if pc.IsUnpacked() {
			t.Error("IsUnpacked() = true, want false for a file (not directory)")
		}
	})

	t.Run("empty path returns false", func(t *testing.T) {
		pc := PlatformConfig{}
		if pc.IsUnpacked() {
			t.Error("IsUnpacked() = true, want false for empty path")
		}
	})
}

func TestPlatformConfig_MarshalYAML(t *testing.T) {
	pc := PlatformConfig{name: "qemu", version: "0.14.0"}
	out, err := pc.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error: %v", err)
	}
	if out != nil {
		t.Errorf("MarshalYAML() = %v, want nil", out)
	}
}

// --- KConfig tests ---

func TestPlatformConfig_KConfig(t *testing.T) {
	tests := []struct {
		name       string
		platName   string
		wantKeys   []string
		wantValues []string
	}{
		{
			name:       "fc platform sets KVM and Firecracker",
			platName:   "fc",
			wantKeys:   []string{"CONFIG_PLAT_KVM", "CONFIG_KVM_VMM_FIRECRACKER"},
			wantValues: []string{kconfig.Yes, kconfig.Yes},
		},
		{
			name:       "firecracker alias sets KVM and Firecracker",
			platName:   "firecracker",
			wantKeys:   []string{"CONFIG_PLAT_KVM", "CONFIG_KVM_VMM_FIRECRACKER"},
			wantValues: []string{kconfig.Yes, kconfig.Yes},
		},
		{
			name:       "kraftcloud sets KVM and Firecracker",
			platName:   "kraftcloud",
			wantKeys:   []string{"CONFIG_PLAT_KVM", "CONFIG_KVM_VMM_FIRECRACKER"},
			wantValues: []string{kconfig.Yes, kconfig.Yes},
		},
		{
			name:       "kvm platform sets KVM and QEMU",
			platName:   "kvm",
			wantKeys:   []string{"CONFIG_PLAT_KVM", "CONFIG_KVM_VMM_QEMU"},
			wantValues: []string{kconfig.Yes, kconfig.Yes},
		},
		{
			name:       "qemu platform sets KVM and QEMU",
			platName:   "qemu",
			wantKeys:   []string{"CONFIG_PLAT_KVM", "CONFIG_KVM_VMM_QEMU"},
			wantValues: []string{kconfig.Yes, kconfig.Yes},
		},
		{
			name:       "xen platform sets XEN",
			platName:   "xen",
			wantKeys:   []string{"CONFIG_PLAT_XEN"},
			wantValues: []string{kconfig.Yes},
		},
		{
			name:       "linuxu platform sets LINUXU",
			platName:   "linuxu",
			wantKeys:   []string{"CONFIG_PLAT_LINUXU"},
			wantValues: []string{kconfig.Yes},
		},
		{
			name:     "unknown platform sets no kconfig",
			platName: "unknownplat",
			wantKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := PlatformConfig{name: tt.platName}
			kv := pc.KConfig()
			if kv == nil {
				t.Fatal("KConfig() returned nil")
			}
			for i, key := range tt.wantKeys {
				val, ok := kv[key]
				if !ok {
					t.Errorf("KConfig() missing key %q", key)
					continue
				}
				if val.Value != tt.wantValues[i] {
					t.Errorf("KConfig()[%q] = %q, want %q", key, val.Value, tt.wantValues[i])
				}
			}
		})
	}
}

func TestPlatformConfig_KConfig_overrideByCustom(t *testing.T) {
	customKV, _ := kconfig.NewKeyValueMapFromMap(map[string]interface{}{"CONFIG_CUSTOM": "y"})
	pc := PlatformConfig{
		name:    "qemu",
		kconfig: customKV,
	}

	kv := pc.KConfig()
	if _, ok := kv["CONFIG_CUSTOM"]; !ok {
		t.Error("KConfig() should include custom kconfig value")
	}
	if _, ok := kv["CONFIG_PLAT_KVM"]; !ok {
		t.Error("KConfig() should include built-in CONFIG_PLAT_KVM for qemu")
	}
}

// --- NewPlatformFromOptions tests ---

func TestNewPlatformFromOptions(t *testing.T) {
	t.Run("no options returns empty config", func(t *testing.T) {
		p := NewPlatformFromOptions()
		if p.Name() != "" {
			t.Errorf("Name() = %q, want empty", p.Name())
		}
		if p.Version() != "" {
			t.Errorf("Version() = %q, want empty", p.Version())
		}
	})

	t.Run("with name", func(t *testing.T) {
		p := NewPlatformFromOptions(WithName("qemu"))
		if p.Name() != "qemu" {
			t.Errorf("Name() = %q, want %q", p.Name(), "qemu")
		}
	})

	t.Run("with version", func(t *testing.T) {
		p := NewPlatformFromOptions(WithVersion("0.14.0"))
		if p.Version() != "0.14.0" {
			t.Errorf("Version() = %q, want %q", p.Version(), "0.14.0")
		}
	})

	t.Run("with source", func(t *testing.T) {
		p := NewPlatformFromOptions(WithSource("https://example.com/plat"))
		if p.Source() != "https://example.com/plat" {
			t.Errorf("Source() = %q, want %q", p.Source(), "https://example.com/plat")
		}
	})

	t.Run("with path", func(t *testing.T) {
		p := NewPlatformFromOptions(WithPath("/my/path"))
		if p.Path() != "/my/path" {
			t.Errorf("Path() = %q, want %q", p.Path(), "/my/path")
		}
	})

	t.Run("with internal", func(t *testing.T) {
		p := NewPlatformFromOptions(WithInternal(true))
		pc := p.(*PlatformConfig)
		if !pc.internal {
			t.Error("internal = false, want true")
		}
	})

	t.Run("with kconfig", func(t *testing.T) {
		kv, _ := kconfig.NewKeyValueMapFromMap(map[string]interface{}{"CONFIG_MY_OPT": "y"})
		p := NewPlatformFromOptions(WithKConfig(kv))
		result := p.KConfig()
		if _, ok := result["CONFIG_MY_OPT"]; !ok {
			t.Error("KConfig() missing expected key CONFIG_MY_OPT")
		}
	})

	t.Run("with all options", func(t *testing.T) {
		kv, _ := kconfig.NewKeyValueMapFromMap(map[string]interface{}{"CONFIG_X": "y"})
		p := NewPlatformFromOptions(
			WithName("xen"),
			WithVersion("1.0"),
			WithSource("https://example.com/xen"),
			WithPath("/plat/xen"),
			WithInternal(true),
			WithKConfig(kv),
		)
		if p.Name() != "xen" {
			t.Errorf("Name() = %q, want %q", p.Name(), "xen")
		}
		if p.Version() != "1.0" {
			t.Errorf("Version() = %q, want %q", p.Version(), "1.0")
		}
		if p.Source() != "https://example.com/xen" {
			t.Errorf("Source() = %q, want %q", p.Source(), "https://example.com/xen")
		}
		if p.Path() != "/plat/xen" {
			t.Errorf("Path() = %q, want %q", p.Path(), "/plat/xen")
		}
		if p.Type() != unikraft.ComponentTypePlat {
			t.Errorf("Type() = %v, want %v", p.Type(), unikraft.ComponentTypePlat)
		}
	})
}

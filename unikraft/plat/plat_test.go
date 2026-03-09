// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package plat

import (
	"context"
	"os"
	"testing"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
)

// ---------------------------------------------------------------------------
// NewPlatformFromOptions
// ---------------------------------------------------------------------------

func TestNewPlatformFromOptions(t *testing.T) {
	kv := kconfig.KeyValueMap{}
	kv.Set("CONFIG_TEST", kconfig.Yes)

	pc := NewPlatformFromOptions(
		WithName("kvm"),
		WithVersion("0.15.0"),
		WithSource("https://github.com/unikraft/unikraft"),
		WithPath("/tmp/unikraft"),
		WithInternal(true),
		WithKConfig(kv),
	)

	if pc.Name() != "kvm" {
		t.Errorf("Name() = %q, want %q", pc.Name(), "kvm")
	}
	if pc.Version() != "0.15.0" {
		t.Errorf("Version() = %q, want %q", pc.Version(), "0.15.0")
	}
	if pc.Source() != "https://github.com/unikraft/unikraft" {
		t.Errorf("Source() = %q, want %q", pc.Source(), "https://github.com/unikraft/unikraft")
	}
	if pc.Path() != "/tmp/unikraft" {
		t.Errorf("Path() = %q, want %q", pc.Path(), "/tmp/unikraft")
	}
	if pc.Type() != unikraft.ComponentTypePlat {
		t.Errorf("Type() = %v, want %v", pc.Type(), unikraft.ComponentTypePlat)
	}
	if pc.String() != "kvm" {
		t.Errorf("String() = %q, want %q", pc.String(), "kvm")
	}
}

// ---------------------------------------------------------------------------
// IsUnpacked
// ---------------------------------------------------------------------------

func TestPlatformConfig_IsUnpacked(t *testing.T) {
	t.Run("existing directory returns true", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "plat-test-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		pc := PlatformConfig{path: dir}
		if !pc.IsUnpacked() {
			t.Errorf("IsUnpacked() = false, want true for existing dir")
		}
	})

	t.Run("non-existent path returns false", func(t *testing.T) {
		pc := PlatformConfig{path: "/nonexistent/path/xyz"}
		if pc.IsUnpacked() {
			t.Errorf("IsUnpacked() = true, want false for non-existent path")
		}
	})

	t.Run("empty path returns false", func(t *testing.T) {
		pc := PlatformConfig{}
		if pc.IsUnpacked() {
			t.Errorf("IsUnpacked() = true, want false for empty path")
		}
	})
}

// ---------------------------------------------------------------------------
// KConfigTree
// ---------------------------------------------------------------------------

func TestPlatformConfig_KConfigTree(t *testing.T) {
	pc := PlatformConfig{name: "kvm"}
	tree, err := pc.KConfigTree(context.Background())
	if err != nil {
		t.Errorf("KConfigTree() unexpected error = %v", err)
	}
	if tree != nil {
		t.Errorf("KConfigTree() = %v, want nil", tree)
	}
}

// ---------------------------------------------------------------------------
// MarshalYAML
// ---------------------------------------------------------------------------

func TestPlatformConfig_MarshalYAML(t *testing.T) {
	pc := PlatformConfig{name: "kvm"}
	got, err := pc.MarshalYAML()
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

func TestPlatformConfig_PrintInfo(t *testing.T) {
	pc := PlatformConfig{name: "kvm"}
	got := pc.PrintInfo(context.Background())
	if got == "" {
		t.Errorf("PrintInfo() returned empty string")
	}
}

// ---------------------------------------------------------------------------
// KConfig
// ---------------------------------------------------------------------------

func TestPlatformConfig_KConfig(t *testing.T) {
	tests := []struct {
		platName string
		wantKeys []string
		noKeys   []string
	}{
		{
			platName: "fc",
			wantKeys: []string{"CONFIG_PLAT_KVM", "CONFIG_KVM_VMM_FIRECRACKER"},
		},
		{
			platName: "firecracker",
			wantKeys: []string{"CONFIG_PLAT_KVM", "CONFIG_KVM_VMM_FIRECRACKER"},
		},
		{
			platName: "kraftcloud",
			wantKeys: []string{"CONFIG_PLAT_KVM", "CONFIG_KVM_VMM_FIRECRACKER"},
		},
		{
			platName: "kvm",
			wantKeys: []string{"CONFIG_PLAT_KVM", "CONFIG_KVM_VMM_QEMU"},
		},
		{
			platName: "qemu",
			wantKeys: []string{"CONFIG_PLAT_KVM", "CONFIG_KVM_VMM_QEMU"},
		},
		{
			platName: "xen",
			wantKeys: []string{"CONFIG_PLAT_XEN"},
			noKeys:   []string{"CONFIG_PLAT_KVM"},
		},
		{
			platName: "linuxu",
			wantKeys: []string{"CONFIG_PLAT_LINUXU"},
			noKeys:   []string{"CONFIG_PLAT_KVM"},
		},
		{
			platName: "unknown",
			noKeys:   []string{"CONFIG_PLAT_KVM", "CONFIG_PLAT_XEN", "CONFIG_PLAT_LINUXU"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.platName, func(t *testing.T) {
			pc := PlatformConfig{name: tt.platName}
			kconf := pc.KConfig()

			for _, key := range tt.wantKeys {
				if _, exists := kconf.Get(key); !exists {
					t.Errorf("KConfig() missing key %q for platform %q", key, tt.platName)
				}
			}
			for _, key := range tt.noKeys {
				if _, exists := kconf.Get(key); exists {
					t.Errorf("KConfig() unexpected key %q for platform %q", key, tt.platName)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TransformFromSchema
// ---------------------------------------------------------------------------

func TestTransformFromSchema(t *testing.T) {
	t.Run("string input sets name", func(t *testing.T) {
		got, err := TransformFromSchema(context.Background(), "kvm")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pc := got.(PlatformConfig)
		if pc.Name() != "qemu" {
			// "kvm" is an alias for "qemu" via PlatformsByName
			t.Errorf("Name() = %q, want %q", pc.Name(), "qemu")
		}
	})

	t.Run("firecracker alias rewritten to fc", func(t *testing.T) {
		got, err := TransformFromSchema(context.Background(), "firecracker")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pc := got.(PlatformConfig)
		if pc.Name() != "fc" {
			t.Errorf("Name() = %q, want %q (alias rewrite)", pc.Name(), "fc")
		}
	})

	t.Run("unknown platform name kept as-is", func(t *testing.T) {
		got, err := TransformFromSchema(context.Background(), "linuxu")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pc := got.(PlatformConfig)
		if pc.Name() != "linuxu" {
			t.Errorf("Name() = %q, want %q", pc.Name(), "linuxu")
		}
	})

	t.Run("non-string input errors", func(t *testing.T) {
		_, err := TransformFromSchema(context.Background(), 123)
		if err == nil {
			t.Errorf("expected error for non-string input")
		}
	})

	t.Run("map input errors", func(t *testing.T) {
		_, err := TransformFromSchema(context.Background(), map[string]interface{}{"name": "kvm"})
		if err == nil {
			t.Errorf("expected error for map input")
		}
	})

	t.Run("with UK_BASE context sets path", func(t *testing.T) {
		ctx := unikraft.WithContext(context.Background(), &unikraft.Context{
			UK_BASE: "/tmp/unikraft-base",
		})
		got, err := TransformFromSchema(ctx, "xen")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pc := got.(PlatformConfig)
		if pc.Path() == "" {
			t.Errorf("expected path to be set when UK_BASE provided")
		}
	})
}

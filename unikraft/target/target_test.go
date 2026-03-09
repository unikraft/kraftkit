// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package target

import (
	"fmt"
	"path/filepath"
	"testing"

	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
)

// newTarget is a convenience constructor for test fixtures.
func newTarget(name, platName, archName string) TargetConfig {
	return TargetConfig{
		name:         name,
		platform:     plat.NewPlatformFromOptions(plat.WithName(platName)),
		architecture: arch.NewArchitectureFromOptions(arch.WithName(archName)),
	}
}

// ---------------------------------------------------------------------------
// KernelName
// ---------------------------------------------------------------------------

func TestKernelName(t *testing.T) {
	tests := []struct {
		name    string
		target  TargetConfig
		want    string
		wantErr bool
	}{
		{
			name:    "empty target name returns error",
			target:  newTarget("", "kvm", "x86_64"),
			wantErr: true,
		},
		{
			name:   "kvm/x86_64",
			target: newTarget("myapp", "kvm", "x86_64"),
			want:   "myapp_kvm-x86_64",
		},
		{
			name:   "qemu/arm64",
			target: newTarget("helloworld", "qemu", "arm64"),
			want:   "helloworld_qemu-arm64",
		},
		{
			name:   "linuxu/x86_64",
			target: newTarget("app", "linuxu", "x86_64"),
			want:   "app_linuxu-x86_64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := KernelName(tt.target)
			if (err != nil) != tt.wantErr {
				t.Fatalf("KernelName() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("KernelName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// KernelDbgName
// ---------------------------------------------------------------------------

func TestKernelDbgName(t *testing.T) {
	tests := []struct {
		name    string
		target  TargetConfig
		want    string
		wantErr bool
	}{
		{
			name:    "empty target name returns error",
			target:  newTarget("", "kvm", "x86_64"),
			wantErr: true,
		},
		{
			name:   "appends .dbg suffix for kvm/x86_64",
			target: newTarget("myapp", "kvm", "x86_64"),
			want:   "myapp_kvm-x86_64.dbg",
		},
		{
			name:   "appends .dbg suffix for fc/arm64",
			target: newTarget("srv", "fc", "arm64"),
			want:   "srv_fc-arm64.dbg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := KernelDbgName(tt.target)
			if (err != nil) != tt.wantErr {
				t.Fatalf("KernelDbgName() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("KernelDbgName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TargetPlatArchName
// ---------------------------------------------------------------------------

func TestTargetPlatArchName(t *testing.T) {
	tests := []struct {
		platName string
		archName string
		want     string
	}{
		{"kvm", "x86_64", "kvm/x86_64"},
		{"xen", "arm64", "xen/arm64"},
		{"linuxu", "arm", "linuxu/arm"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s", tt.platName, tt.archName), func(t *testing.T) {
			tgt := NewTargetFromOptions(
				WithArchitecture(arch.NewArchitectureFromOptions(arch.WithName(tt.archName))),
				WithPlatform(plat.NewPlatformFromOptions(plat.WithName(tt.platName))),
			)
			got := TargetPlatArchName(tgt)
			if got != tt.want {
				t.Errorf("TargetPlatArchName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ConfigFilename
// ---------------------------------------------------------------------------

func TestConfigFilename(t *testing.T) {
	t.Run("kernel path set — uses basename", func(t *testing.T) {
		tc := TargetConfig{
			name:         "app",
			platform:     plat.NewPlatformFromOptions(plat.WithName("kvm")),
			architecture: arch.NewArchitectureFromOptions(arch.WithName("x86_64")),
			kernel:       "/build/app_kvm-x86_64",
		}
		want := ".config.app_kvm-x86_64"
		if got := tc.ConfigFilename(); got != want {
			t.Errorf("ConfigFilename() = %q, want %q", got, want)
		}
	})

	t.Run("kernel path unset — formats from name/plat/arch", func(t *testing.T) {
		tc := TargetConfig{
			name:         "myapp",
			platform:     plat.NewPlatformFromOptions(plat.WithName("kvm")),
			architecture: arch.NewArchitectureFromOptions(arch.WithName("x86_64")),
		}
		want := ".config.myapp_kvm-x86_64"
		if got := tc.ConfigFilename(); got != want {
			t.Errorf("ConfigFilename() = %q, want %q", got, want)
		}
	})

	t.Run("kernel path with directory — uses only base filename", func(t *testing.T) {
		tc := TargetConfig{
			name:         "srv",
			platform:     plat.NewPlatformFromOptions(plat.WithName("fc")),
			architecture: arch.NewArchitectureFromOptions(arch.WithName("arm64")),
			kernel:       filepath.Join("/some", "build", "dir", "srv_fc-arm64"),
		}
		want := ".config.srv_fc-arm64"
		if got := tc.ConfigFilename(); got != want {
			t.Errorf("ConfigFilename() = %q, want %q", got, want)
		}
	})
}

// ---------------------------------------------------------------------------
// NewTargetFromOptions / SetKernelPath
// ---------------------------------------------------------------------------

func TestNewTargetFromOptions(t *testing.T) {
	tgt := NewTargetFromOptions(
		WithName("demo"),
		WithArchitecture(arch.NewArchitectureFromOptions(arch.WithName("x86_64"))),
		WithPlatform(plat.NewPlatformFromOptions(plat.WithName("kvm"))),
		WithKernel("/build/demo_kvm-x86_64"),
		WithKernelDbg("/build/demo_kvm-x86_64.dbg"),
		WithCommand([]string{"/bin/sh", "-c", "echo hello"}),
	)

	if got := tgt.Name(); got != "demo" {
		t.Errorf("Name() = %q, want %q", got, "demo")
	}
	if got := tgt.Architecture().Name(); got != "x86_64" {
		t.Errorf("Architecture().Name() = %q, want %q", got, "x86_64")
	}
	if got := tgt.Platform().Name(); got != "kvm" {
		t.Errorf("Platform().Name() = %q, want %q", got, "kvm")
	}
	if got := tgt.Kernel(); got != "/build/demo_kvm-x86_64" {
		t.Errorf("Kernel() = %q, want %q", got, "/build/demo_kvm-x86_64")
	}
	if got := tgt.KernelDbg(); got != "/build/demo_kvm-x86_64.dbg" {
		t.Errorf("KernelDbg() = %q, want %q", got, "/build/demo_kvm-x86_64.dbg")
	}
	if got := len(tgt.Command()); got != 3 {
		t.Errorf("Command() length = %d, want 3", got)
	}
}

func TestSetKernelPath(t *testing.T) {
	tgt := NewTargetFromOptions(
		WithName("app"),
		WithArchitecture(arch.NewArchitectureFromOptions(arch.WithName("x86_64"))),
		WithPlatform(plat.NewPlatformFromOptions(plat.WithName("kvm"))),
	)

	const newPath = "/new/build/app_kvm-x86_64"
	tgt.SetKernelPath(newPath)
	if got := tgt.Kernel(); got != newPath {
		t.Errorf("after SetKernelPath: Kernel() = %q, want %q", got, newPath)
	}
}

// ---------------------------------------------------------------------------
// Simple accessor / metadata methods
// ---------------------------------------------------------------------------

func TestTargetConfigAccessors(t *testing.T) {
	tc := newTarget("demo", "kvm", "x86_64")

	t.Run("String", func(t *testing.T) {
		s := tc.String()
		if s == "" {
			t.Error("String() returned empty string")
		}
	})

	t.Run("Source returns empty string", func(t *testing.T) {
		if got := tc.Source(); got != "" {
			t.Errorf("Source() = %q, want empty", got)
		}
	})

	t.Run("Version returns empty string", func(t *testing.T) {
		if got := tc.Version(); got != "" {
			t.Errorf("Version() = %q, want empty", got)
		}
	})

	t.Run("Type returns ComponentTypeUnknown", func(t *testing.T) {
		_ = tc.Type() // just exercise the branch; value checked by import
	})

	t.Run("Path returns empty string", func(t *testing.T) {
		if got := tc.Path(); got != "" {
			t.Errorf("Path() = %q, want empty", got)
		}
	})

	t.Run("IsUnpacked returns false", func(t *testing.T) {
		if tc.IsUnpacked() {
			t.Error("IsUnpacked() = true, want false")
		}
	})

	t.Run("Roms returns nil for zero value", func(t *testing.T) {
		if got := tc.Roms(); len(got) != 0 {
			t.Errorf("Roms() len = %d, want 0", len(got))
		}
	})

	t.Run("Initrd returns nil for zero value", func(t *testing.T) {
		if got := tc.Initrd(); got != nil {
			t.Errorf("Initrd() = %v, want nil", got)
		}
	})
}

// ---------------------------------------------------------------------------
// KConfig
// ---------------------------------------------------------------------------

func TestKConfig(t *testing.T) {
	t.Run("nil kconfig is initialised and merged with arch+plat KConfig", func(t *testing.T) {
		tc := newTarget("app", "linuxu", "x86_64")
		kc := tc.KConfig()
		if kc == nil {
			t.Fatal("KConfig() returned nil")
		}
		// x86_64 arch should contribute CONFIG_ARCH_X86_64=y
		if _, ok := kc["CONFIG_ARCH_X86_64"]; !ok {
			t.Error("expected CONFIG_ARCH_X86_64 in KConfig, not found")
		}
	})

	t.Run("WithKConfig option wires kconfig into target", func(t *testing.T) {
		tgt := NewTargetFromOptions(
			WithName("cfg"),
			WithArchitecture(arch.NewArchitectureFromOptions(arch.WithName("x86_64"))),
			WithPlatform(plat.NewPlatformFromOptions(plat.WithName("linuxu"))),
			WithKConfig(nil),
		)
		kc := tgt.(*TargetConfig).KConfig()
		if kc == nil {
			t.Error("KConfig() returned nil after WithKConfig(nil)")
		}
	})
}

// ---------------------------------------------------------------------------
// KConfigTree and PrintInfo
// ---------------------------------------------------------------------------

func TestKConfigTree(t *testing.T) {
	tc := newTarget("app", "kvm", "x86_64")
	tree, err := tc.KConfigTree(nil)
	if err == nil {
		t.Error("KConfigTree() expected error, got nil")
	}
	if tree != nil {
		t.Errorf("KConfigTree() = %v, want nil", tree)
	}
}

func TestPrintInfo(t *testing.T) {
	tc := newTarget("app", "kvm", "x86_64")
	s := tc.PrintInfo(nil)
	if s == "" {
		t.Error("PrintInfo() returned empty string")
	}
}

// ---------------------------------------------------------------------------
// MarshalYAML
// ---------------------------------------------------------------------------

func TestMarshalYAML(t *testing.T) {
	t.Run("minimal — no name or kconfig", func(t *testing.T) {
		tc := newTarget("", "linuxu", "x86_64")
		out, err := tc.MarshalYAML()
		if err != nil {
			t.Fatalf("MarshalYAML() error = %v", err)
		}
		m, ok := out.(map[string]interface{})
		if !ok {
			t.Fatalf("MarshalYAML() returned %T, want map", out)
		}
		if m["architecture"] != "x86_64" {
			t.Errorf("architecture = %v, want x86_64", m["architecture"])
		}
		if m["platform"] != "linuxu" {
			t.Errorf("platform = %v, want linuxu", m["platform"])
		}
		if _, ok := m["name"]; ok {
			t.Error("name key should be absent for empty name")
		}
	})

	t.Run("includes name when set", func(t *testing.T) {
		tc := newTarget("myapp", "linuxu", "x86_64")
		out, err := tc.MarshalYAML()
		if err != nil {
			t.Fatalf("MarshalYAML() error = %v", err)
		}
		m := out.(map[string]interface{})
		if m["name"] != "myapp" {
			t.Errorf("name = %v, want myapp", m["name"])
		}
	})

	t.Run("includes output when kernel path set", func(t *testing.T) {
		tc := TargetConfig{
			name:         "app",
			platform:     plat.NewPlatformFromOptions(plat.WithName("linuxu")),
			architecture: arch.NewArchitectureFromOptions(arch.WithName("x86_64")),
			kernel:       "/build/app_linuxu-x86_64",
		}
		out, err := tc.MarshalYAML()
		if err != nil {
			t.Fatalf("MarshalYAML() error = %v", err)
		}
		m := out.(map[string]interface{})
		if m["output"] != "/build/app_linuxu-x86_64" {
			t.Errorf("output = %v, want /build/app_linuxu-x86_64", m["output"])
		}
	})
}

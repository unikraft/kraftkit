// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
)

// ---------------------------------------------------------------------------
// constants — args.go
// ---------------------------------------------------------------------------

func TestConstants(t *testing.T) {
	if MakeDelimeter != ":" {
		t.Errorf("MakeDelimeter = %q, want %q", MakeDelimeter, ":")
	}
	if VerboseQuiet != "0" {
		t.Errorf("VerboseQuiet = %q, want %q", VerboseQuiet, "0")
	}
	if VerboseBuild != "1" {
		t.Errorf("VerboseBuild = %q, want %q", VerboseBuild, "1")
	}
	if VerboseExtra != "2" {
		t.Errorf("VerboseExtra = %q, want %q", VerboseExtra, "2")
	}
}

// ---------------------------------------------------------------------------
// NewUnikraftFromOptions
// ---------------------------------------------------------------------------

func TestNewUnikraftFromOptions(t *testing.T) {
	t.Run("empty context no options", func(t *testing.T) {
		uc, err := NewUnikraftFromOptions(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if uc == nil {
			t.Fatal("expected non-nil UnikraftConfig")
		}
	})

	t.Run("with all options", func(t *testing.T) {
		uc, err := NewUnikraftFromOptions(
			context.Background(),
			WithVersion("0.15.0"),
			WithSource("https://github.com/unikraft/unikraft"),
			WithPath("/tmp/unikraft"),
			WithLicense("BSD-3-Clause"),
			WithCompiler("gcc"),
			WithCompileDate("2024-01-01"),
			WithCompiledBy("user"),
			WithCompiledByAssoc("org"),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if uc.Version() != "0.15.0" {
			t.Errorf("Version() = %q, want %q", uc.Version(), "0.15.0")
		}
		if uc.Source() != "https://github.com/unikraft/unikraft" {
			t.Errorf("Source() = %q, want %q", uc.Source(), "https://github.com/unikraft/unikraft")
		}
		if uc.Path() != "/tmp/unikraft" {
			t.Errorf("Path() = %q, want %q", uc.Path(), "/tmp/unikraft")
		}
		if uc.License() != "BSD-3-Clause" {
			t.Errorf("License() = %q, want %q", uc.License(), "BSD-3-Clause")
		}
		if uc.Compiler() != "gcc" {
			t.Errorf("Compiler() = %q, want %q", uc.Compiler(), "gcc")
		}
		if uc.CompileDate() != "2024-01-01" {
			t.Errorf("CompileDate() = %q, want %q", uc.CompileDate(), "2024-01-01")
		}
		if uc.CompiledBy() != "user" {
			t.Errorf("CompiledBy() = %q, want %q", uc.CompiledBy(), "user")
		}
		if uc.CompiledByAssoc() != "org" {
			t.Errorf("CompiledByAssoc() = %q, want %q", uc.CompiledByAssoc(), "org")
		}
	})

	t.Run("with UK_BASE context sets path", func(t *testing.T) {
		ctx := unikraft.WithContext(context.Background(), &unikraft.Context{
			UK_BASE: "/tmp/unikraft-base",
		})
		uc, err := NewUnikraftFromOptions(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if uc.Path() == "" {
			t.Errorf("expected path to be set when UK_BASE is provided")
		}
	})
}

// ---------------------------------------------------------------------------
// UnikraftConfig accessors
// ---------------------------------------------------------------------------

func TestUnikraftConfig_Accessors(t *testing.T) {
	uc := UnikraftConfig{}

	if uc.Name() != "unikraft" {
		t.Errorf("Name() = %q, want %q", uc.Name(), "unikraft")
	}
	if uc.String() != "unikraft" {
		t.Errorf("String() = %q, want %q", uc.String(), "unikraft")
	}
	if uc.Type() != unikraft.ComponentTypeCore {
		t.Errorf("Type() = %v, want %v", uc.Type(), unikraft.ComponentTypeCore)
	}
}

// ---------------------------------------------------------------------------
// IsUnpacked
// ---------------------------------------------------------------------------

func TestUnikraftConfig_IsUnpacked(t *testing.T) {
	t.Run("existing directory returns true", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "core-test-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		uc := UnikraftConfig{path: dir}
		if !uc.IsUnpacked() {
			t.Errorf("IsUnpacked() = false, want true for existing dir")
		}
	})

	t.Run("non-existent path returns false", func(t *testing.T) {
		uc := UnikraftConfig{path: "/nonexistent/path/xyz"}
		if uc.IsUnpacked() {
			t.Errorf("IsUnpacked() = true, want false for non-existent path")
		}
	})

	t.Run("empty path returns false", func(t *testing.T) {
		uc := UnikraftConfig{}
		if uc.IsUnpacked() {
			t.Errorf("IsUnpacked() = true, want false for empty path")
		}
	})
}

// ---------------------------------------------------------------------------
// KConfig
// ---------------------------------------------------------------------------

func TestUnikraftConfig_KConfig(t *testing.T) {
	t.Run("nil kconfig returns empty map", func(t *testing.T) {
		uc := UnikraftConfig{}
		kconf := uc.KConfig()
		if kconf == nil {
			t.Errorf("KConfig() = nil, want empty map")
		}
		if len(kconf) != 0 {
			t.Errorf("KConfig() len = %d, want 0", len(kconf))
		}
	})

	t.Run("set kconfig returned correctly", func(t *testing.T) {
		kv := kconfig.KeyValueMap{}
		kv.Set("CONFIG_TEST", kconfig.Yes)
		uc := UnikraftConfig{kconfig: kv}
		got := uc.KConfig()
		if _, exists := got.Get("CONFIG_TEST"); !exists {
			t.Errorf("KConfig() missing CONFIG_TEST")
		}
	})
}

// ---------------------------------------------------------------------------
// CONFIG_* path helpers
// ---------------------------------------------------------------------------

func TestUnikraftConfig_PathHelpers(t *testing.T) {
	uc := UnikraftConfig{path: "/tmp/unikraft"}

	tests := []struct {
		name string
		fn   func() (string, error)
		want string
	}{
		{
			name: "CONFIG_UK_PLAT",
			fn:   uc.CONFIG_UK_PLAT,
			want: filepath.Join("/tmp/unikraft", CONFIG_UK_PLAT),
		},
		{
			name: "CONFIG_UK_LIB",
			fn:   uc.CONFIG_UK_LIB,
			want: filepath.Join("/tmp/unikraft", CONFIG_UK_LIB),
		},
		{
			name: "CONFIG_CONFIG_IN",
			fn:   uc.CONFIG_CONFIG_IN,
			want: filepath.Join("/tmp/unikraft", unikraft.Config_uk),
		},
		{
			name: "CONFIG",
			fn:   uc.CONFIG,
			want: filepath.Join("/tmp/unikraft", CONFIG),
		},
		{
			name: "CONFIGLIB",
			fn:   uc.CONFIGLIB,
			want: filepath.Join("/tmp/unikraft", CONFIGLIB),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn()
			if err != nil {
				t.Errorf("%s() unexpected error: %v", tt.name, err)
			}
			if got != tt.want {
				t.Errorf("%s() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// PrintInfo
// ---------------------------------------------------------------------------

func TestUnikraftConfig_PrintInfo(t *testing.T) {
	uc := UnikraftConfig{}
	got := uc.PrintInfo(context.Background())
	if got == "" {
		t.Errorf("PrintInfo() returned empty string")
	}
}

// ---------------------------------------------------------------------------
// KConfigTree
// ---------------------------------------------------------------------------

func TestUnikraftConfig_KConfigTree_MissingFile(t *testing.T) {
	uc := UnikraftConfig{path: "/nonexistent/path"}
	_, err := uc.KConfigTree(context.Background())
	if err == nil {
		t.Errorf("KConfigTree() expected error for missing Config.uk, got nil")
	}
}

// ---------------------------------------------------------------------------
// MarshalYAML
// ---------------------------------------------------------------------------

func TestUnikraftConfig_MarshalYAML(t *testing.T) {
	t.Run("without kconfig", func(t *testing.T) {
		uc := UnikraftConfig{version: "0.15.0"}
		got, err := uc.MarshalYAML()
		if err != nil {
			t.Fatalf("MarshalYAML() unexpected error: %v", err)
		}
		result := got.(map[string]interface{})
		if result["version"] != "0.15.0" {
			t.Errorf("MarshalYAML()[version] = %v, want %q", result["version"], "0.15.0")
		}
		if _, ok := result["kconfig"]; ok {
			t.Errorf("MarshalYAML() should not include kconfig when empty")
		}
	})

	t.Run("with kconfig", func(t *testing.T) {
		kv := kconfig.KeyValueMap{}
		kv.Set("CONFIG_TEST", kconfig.Yes)
		uc := UnikraftConfig{version: "0.15.0", kconfig: kv}
		got, err := uc.MarshalYAML()
		if err != nil {
			t.Fatalf("MarshalYAML() unexpected error: %v", err)
		}
		result := got.(map[string]interface{})
		if result["kconfig"] == nil {
			t.Errorf("MarshalYAML() kconfig should be present")
		}
	})
}

// ---------------------------------------------------------------------------
// TransformFromSchema
// ---------------------------------------------------------------------------

func TestTransformFromSchema(t *testing.T) {
	t.Run("version string input", func(t *testing.T) {
		got, err := TransformFromSchema(context.Background(), "0.15.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		uc := got.(UnikraftConfig)
		if uc.Version() != "0.15.0" {
			t.Errorf("Version() = %q, want %q", uc.Version(), "0.15.0")
		}
	})

	t.Run("map with version", func(t *testing.T) {
		got, err := TransformFromSchema(context.Background(), map[string]interface{}{
			"version": "0.15.0",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		uc := got.(UnikraftConfig)
		if uc.Version() != "0.15.0" {
			t.Errorf("Version() = %q, want %q", uc.Version(), "0.15.0")
		}
	})

	t.Run("map with URL source", func(t *testing.T) {
		got, err := TransformFromSchema(context.Background(), map[string]interface{}{
			"source": "https://github.com/unikraft/unikraft",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		uc := got.(UnikraftConfig)
		if uc.Source() != "https://github.com/unikraft/unikraft" {
			t.Errorf("Source() = %q, want %q", uc.Source(), "https://github.com/unikraft/unikraft")
		}
	})

	t.Run("map with local dir source", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "core-transform-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		got, err := TransformFromSchema(context.Background(), map[string]interface{}{
			"source": dir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		uc := got.(UnikraftConfig)
		if uc.Path() != dir {
			t.Errorf("Path() = %q, want %q", uc.Path(), dir)
		}
	})

	t.Run("map with local dir source and UK_BASE context", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "core-transform-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		ctx := unikraft.WithContext(context.Background(), &unikraft.Context{
			UK_BASE: "/tmp/base",
		})
		_, err = TransformFromSchema(ctx, map[string]interface{}{
			"source": dir,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("map with kconfig", func(t *testing.T) {
		got, err := TransformFromSchema(context.Background(), map[string]interface{}{
			"kconfig": []interface{}{"CONFIG_TEST=y"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		uc := got.(UnikraftConfig)
		if _, exists := uc.KConfig().Get("CONFIG_TEST"); !exists {
			t.Errorf("KConfig() missing CONFIG_TEST")
		}
	})

	t.Run("invalid source type errors", func(t *testing.T) {
		_, err := TransformFromSchema(context.Background(), map[string]interface{}{
			"source": 123,
		})
		if err == nil {
			t.Errorf("expected error for non-string source")
		}
	})

	t.Run("with UK_BASE sets path", func(t *testing.T) {
		ctx := unikraft.WithContext(context.Background(), &unikraft.Context{
			UK_BASE: "/tmp/unikraft-base",
		})
		got, err := TransformFromSchema(ctx, "0.15.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		uc := got.(UnikraftConfig)
		if uc.Path() == "" {
			t.Errorf("expected path to be set when UK_BASE provided")
		}
	})

	t.Run("map with version as int converted to string", func(t *testing.T) {
		got, err := TransformFromSchema(context.Background(), map[string]any{
			"version": 15,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		uc := got.(UnikraftConfig)
		if uc.Version() != "15" {
			t.Errorf("Version() = %q, want %q", uc.Version(), "15")
		}
	})
}

// ---------------------------------------------------------------------------
// Libraries
// ---------------------------------------------------------------------------

func TestUnikraftConfig_Libraries_NonExistentPath(t *testing.T) {
	uc := UnikraftConfig{path: "/nonexistent/path"}
	libs, err := uc.Libraries(context.Background())
	if err != nil {
		t.Fatalf("Libraries() unexpected error: %v", err)
	}
	if libs != nil {
		t.Errorf("Libraries() = %v, want nil for non-existent lib dir", libs)
	}
}

func TestUnikraftConfig_Libraries_EmptyDirs(t *testing.T) {
	// Create a minimal dir structure: <root>/lib/ and <root>/plat/
	root, err := os.MkdirTemp("", "core-libs-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	if err := os.MkdirAll(filepath.Join(root, CONFIG_UK_LIB), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, CONFIG_UK_PLAT), 0o755); err != nil {
		t.Fatal(err)
	}

	uc := UnikraftConfig{path: root}
	libs, err := uc.Libraries(context.Background())
	if err != nil {
		t.Fatalf("Libraries() unexpected error: %v", err)
	}
	if len(libs) != 0 {
		t.Errorf("Libraries() len = %d, want 0 for empty dirs", len(libs))
	}
}

// ---------------------------------------------------------------------------
// WithKConfig option
// ---------------------------------------------------------------------------

func TestWithKConfig(t *testing.T) {
	kv := kconfig.KeyValueMap{}
	kv.Set("CONFIG_TEST", kconfig.Yes)

	uc, err := NewUnikraftFromOptions(context.Background(), WithKConfig(kv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := uc.KConfig().Get("CONFIG_TEST"); !exists {
		t.Errorf("KConfig() missing CONFIG_TEST after WithKConfig")
	}
}

// ---------------------------------------------------------------------------
// NewUnikraftFromOptions — option returning error
// ---------------------------------------------------------------------------

func TestNewUnikraftFromOptions_OptionError(t *testing.T) {
	errOpt := func(*UnikraftConfig) error {
		return os.ErrPermission
	}
	_, err := NewUnikraftFromOptions(context.Background(), errOpt)
	if err == nil {
		t.Errorf("expected error when option returns error")
	}
}

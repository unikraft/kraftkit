// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package config

import "testing"

func TestConfigManagerUnset_MapField(t *testing.T) {
	unset := func(t *testing.T, cfg *KraftKit, key string) {
		t.Helper()
		cm := &ConfigManager[KraftKit]{Config: cfg}
		if err := cm.Unset(key); err != nil {
			t.Fatalf("Unset(%q) returned unexpected error: %v", key, err)
		}
	}

	t.Run("Remove existing map key", func(t *testing.T) {
		cfg := &KraftKit{Toolchain: map[string]string{"CC": "clang", "UK_CFLAGS": "-O2"}}
		unset(t, cfg, "toolchain.CC")
		if _, ok := cfg.Toolchain["CC"]; ok {
			t.Error("expected toolchain.CC to be removed, but it still exists")
		}
		if expect, got := "-O2", cfg.Toolchain["UK_CFLAGS"]; expect != got {
			t.Errorf("Toolchain[UK_CFLAGS]: expected %q, got %q", expect, got)
		}
	})

	t.Run("Unset on nil map is a no-op", func(t *testing.T) {
		cfg := &KraftKit{}
		unset(t, cfg, "toolchain.CC")
		if cfg.Toolchain != nil {
			t.Error("expected Toolchain to remain nil")
		}
	})

	t.Run("Unset string field resets to zero value", func(t *testing.T) {
		cfg := &KraftKit{Editor: "vim"}
		unset(t, cfg, "editor")
		if cfg.Editor != "" {
			t.Errorf("Editor: expected empty string, got %q", cfg.Editor)
		}
	})

	t.Run("Unset nested struct field resets to zero value", func(t *testing.T) {
		cfg := &KraftKit{}
		cfg.Log.Level = "debug"
		unset(t, cfg, "log.level")
		if cfg.Log.Level != "" {
			t.Errorf("Log.Level: expected empty string, got %q", cfg.Log.Level)
		}
	})

	t.Run("Error on unknown key", func(t *testing.T) {
		cfg := &KraftKit{}
		cm := &ConfigManager[KraftKit]{Config: cfg}
		if err := cm.Unset("nonexistent"); err == nil {
			t.Error("expected error for unknown key, got nil")
		}
	})

	t.Run("Error on too deep map traversal", func(t *testing.T) {
		cfg := &KraftKit{}
		cm := &ConfigManager[KraftKit]{Config: cfg}
		if err := cm.Unset("toolchain.CC.extra"); err == nil {
			t.Error("expected error for toolchain.CC.extra, got nil")
		}
	})

	t.Run("Remove nested map key", func(t *testing.T) {
		cfg := &KraftKit{
			ToolchainProfiles: map[string]map[string]string{
				"clang-debug": {
					"CC":        "clang",
					"UK_CFLAGS": "-O2",
				},
			},
		}
		unset(t, cfg, "toolchain_profiles.clang-debug.CC")
		if _, ok := cfg.ToolchainProfiles["clang-debug"]["CC"]; ok {
			t.Error("expected toolchain_profiles.clang-debug.CC to be removed")
		}
		if got := cfg.ToolchainProfiles["clang-debug"]["UK_CFLAGS"]; got != "-O2" {
			t.Errorf("expected UK_CFLAGS to be '-O2', got %q", got)
		}
	})

	t.Run("Remove nested map key cleans up profile", func(t *testing.T) {
		cfg := &KraftKit{
			ToolchainProfiles: map[string]map[string]string{
				"clang-debug": {
					"CC": "clang",
				},
			},
		}
		unset(t, cfg, "toolchain_profiles.clang-debug.CC")
		if _, ok := cfg.ToolchainProfiles["clang-debug"]; ok {
			t.Error("expected toolchain_profiles.clang-debug to be removed entirely when empty")
		}
	})

	t.Run("Unset on nested map error on invalid key path length", func(t *testing.T) {
		cfg := &KraftKit{}
		cm := &ConfigManager[KraftKit]{Config: cfg}
		if err := cm.Unset("toolchain_profiles.clang-debug.CC.extra"); err == nil {
			t.Error("expected error for too deep key path, got nil")
		}
	})
}

func TestConfigManagerSet_MapField(t *testing.T) {
	// set is a helper that creates a ConfigManager and calls Set(key, val)
	set := func(t *testing.T, cfg *KraftKit, key, val string) {
		t.Helper()
		cm := &ConfigManager[KraftKit]{Config: cfg}
		if err := cm.Set(key, val); err != nil {
			t.Fatalf("Set(%q, %q) returned unexpected error: %v", key, val, err)
		}
	}

	t.Run("Map field on nil map", func(t *testing.T) {
		cfg := &KraftKit{}
		set(t, cfg, "toolchain.CC", "gcc-12")
		if expect, got := "gcc-12", cfg.Toolchain["CC"]; expect != got {
			t.Errorf("Toolchain[CC]: expected %q, got %q", expect, got)
		}
	})

	t.Run("Map field overwrite", func(t *testing.T) {
		cfg := &KraftKit{Toolchain: map[string]string{"CC": "gcc"}}
		set(t, cfg, "toolchain.CC", "clang")
		if expect, got := "clang", cfg.Toolchain["CC"]; expect != got {
			t.Errorf("Toolchain[CC]: expected %q, got %q", expect, got)
		}
	})

	t.Run("Multiple map keys", func(t *testing.T) {
		cfg := &KraftKit{}
		set(t, cfg, "toolchain.CC", "clang")
		set(t, cfg, "toolchain.UK_CFLAGS", "-O2")
		if expect, got := "clang", cfg.Toolchain["CC"]; expect != got {
			t.Errorf("Toolchain[CC]: expected %q, got %q", expect, got)
		}
		if expect, got := "-O2", cfg.Toolchain["UK_CFLAGS"]; expect != got {
			t.Errorf("Toolchain[UK_CFLAGS]: expected %q, got %q", expect, got)
		}
		if expect, got := 2, len(cfg.Toolchain); expect != got {
			t.Errorf("len(Toolchain): expected %d, got %d", expect, got)
		}
	})

	t.Run("String field", func(t *testing.T) {
		cfg := &KraftKit{}
		set(t, cfg, "editor", "vim")
		if expect, got := "vim", cfg.Editor; expect != got {
			t.Errorf("Editor: expected %q, got %q", expect, got)
		}
	})

	t.Run("Nested struct field", func(t *testing.T) {
		cfg := &KraftKit{}
		set(t, cfg, "log.level", "debug")
		if expect, got := "debug", cfg.Log.Level; expect != got {
			t.Errorf("Log.Level: expected %q, got %q", expect, got)
		}
	})

	// Since the map is map[string]string
	// it can't have sub keys
	// "toolchain.CC" is valid, but "toolchain.CC.extra" must fail
	t.Run("Error on too deep map traversal", func(t *testing.T) {
		cfg := &KraftKit{}
		cm := &ConfigManager[KraftKit]{Config: cfg}
		if err := cm.Set("toolchain.CC.extra", "value"); err == nil {
			t.Error("expected error for toolchain.CC.extra, got nil")
		}
	})

	t.Run("Error on unknown key", func(t *testing.T) {
		cfg := &KraftKit{}
		cm := &ConfigManager[KraftKit]{Config: cfg}
		if err := cm.Set("nonexistent", "value"); err == nil {
			t.Error("expected error for unknown key, got nil")
		}
	})

	t.Run("Nested map field set on nil map", func(t *testing.T) {
		cfg := &KraftKit{}
		set(t, cfg, "toolchain_profiles.clang-debug.CC", "clang")
		if cfg.ToolchainProfiles == nil {
			t.Fatal("expected ToolchainProfiles to be initialized")
		}
		if expect, got := "clang", cfg.ToolchainProfiles["clang-debug"]["CC"]; expect != got {
			t.Errorf("expected CC to be %q, got %q", expect, got)
		}
	})

	t.Run("Nested map field overwrite/addition", func(t *testing.T) {
		cfg := &KraftKit{
			ToolchainProfiles: map[string]map[string]string{
				"clang-debug": {
					"CC": "gcc",
				},
			},
		}
		set(t, cfg, "toolchain_profiles.clang-debug.CC", "clang")
		set(t, cfg, "toolchain_profiles.clang-debug.UK_CFLAGS", "-O2")
		if expect, got := "clang", cfg.ToolchainProfiles["clang-debug"]["CC"]; expect != got {
			t.Errorf("expected CC to be %q, got %q", expect, got)
		}
		if expect, got := "-O2", cfg.ToolchainProfiles["clang-debug"]["UK_CFLAGS"]; expect != got {
			t.Errorf("expected UK_CFLAGS to be %q, got %q", expect, got)
		}
	})

	t.Run("Set on nested map error on invalid key path length", func(t *testing.T) {
		cfg := &KraftKit{}
		cm := &ConfigManager[KraftKit]{Config: cfg}
		if err := cm.Set("toolchain_profiles.clang-debug.CC.extra", "val"); err == nil {
			t.Error("expected error for too deep key path, got nil")
		}
	})
}

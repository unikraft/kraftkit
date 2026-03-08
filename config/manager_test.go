// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package config

import "testing"

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
}

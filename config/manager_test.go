// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package config

import (
	"errors"
	"testing"
)

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
		err := cm.Unset("nonexistent")
		if err == nil {
			t.Fatal("expected error for unknown key, got nil")
		}
		if !errors.Is(err, InvalidKey) {
			t.Fatalf("expected InvalidKey, got %v", err)
		}
	})

	t.Run("Error on too deep map traversal", func(t *testing.T) {
		cfg := &KraftKit{}
		cm := &ConfigManager[KraftKit]{Config: cfg}
		err := cm.Unset("toolchain.CC.extra")
		if err == nil {
			t.Fatal("expected error for toolchain.CC.extra, got nil")
		}
		if !errors.Is(err, CannotTraverseFurtherInMap) {
			t.Fatalf("expected CannotTraverseFurtherInMap, got %v", err)
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
		err := cm.Set("toolchain.CC.extra", "value")
		if err == nil {
			t.Fatal("expected error for toolchain.CC.extra, got nil")
		}
		if !errors.Is(err, CannotTraverseFurtherInMap) {
			t.Fatalf("expected CannotTraverseFurtherInMap, got %v", err)
		}
	})

	t.Run("Error on unknown key", func(t *testing.T) {
		cfg := &KraftKit{}
		cm := &ConfigManager[KraftKit]{Config: cfg}
		err := cm.Set("nonexistent", "value")
		if err == nil {
			t.Fatal("expected error for unknown key, got nil")
		}
		if !errors.Is(err, InvalidKey) {
			t.Fatalf("expected InvalidKey, got %v", err)
		}
	})
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

type writeConfig struct {
	Editor    string            `yaml:"editor,omitempty"`
	Toolchain map[string]string `yaml:"toolchain,omitempty"`
}

func writeFeederFile(t *testing.T, contents string) YamlFeeder {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	return YamlFeeder{File: path}
}

func readFeederFile(t *testing.T, feeder YamlFeeder) writeConfig {
	t.Helper()
	data, err := os.ReadFile(feeder.File)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	var got writeConfig
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal config file: %v", err)
	}
	return got
}

func TestYamlFeederWrite_MergeOverwritesExistingKey(t *testing.T) {
	feeder := writeFeederFile(t, "toolchain:\n  CC: gcc\n")

	if err := feeder.Write(writeConfig{
		Toolchain: map[string]string{"CC": "clang"},
	}, true); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := readFeederFile(t, feeder)
	if expect, have := "clang", got.Toolchain["CC"]; expect != have {
		t.Errorf("toolchain.CC: expected %q, got %q", expect, have)
	}
}

func TestYamlFeederWrite_MergePreservesDiskOnlyKeys(t *testing.T) {
	feeder := writeFeederFile(t, "toolchain:\n  CC: gcc\n  UK_CFLAGS: -O2\n")

	if err := feeder.Write(writeConfig{
		Toolchain: map[string]string{"CC": "clang"},
	}, true); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := readFeederFile(t, feeder)
	if expect, have := "clang", got.Toolchain["CC"]; expect != have {
		t.Errorf("toolchain.CC: expected %q, got %q", expect, have)
	}
	if expect, have := "-O2", got.Toolchain["UK_CFLAGS"]; expect != have {
		t.Errorf("toolchain.UK_CFLAGS: expected %q, got %q", expect, have)
	}
}

func TestYamlFeederWrite_NoMergeDropsMissingKeys(t *testing.T) {
	feeder := writeFeederFile(t, "toolchain:\n  CC: gcc\n  UK_CFLAGS: -O2\n")

	if err := feeder.Write(writeConfig{
		Toolchain: map[string]string{"UK_CFLAGS": "-O2"},
	}, false); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := readFeederFile(t, feeder)
	if _, ok := got.Toolchain["CC"]; ok {
		t.Error("expected toolchain.CC to be removed")
	}
	if expect, have := "-O2", got.Toolchain["UK_CFLAGS"]; expect != have {
		t.Errorf("toolchain.UK_CFLAGS: expected %q, got %q", expect, have)
	}
}

func TestYamlFeederWrite_EmptyFileWritesMemory(t *testing.T) {
	feeder := writeFeederFile(t, "")

	if err := feeder.Write(writeConfig{
		Editor:    "vim",
		Toolchain: map[string]string{"CC": "clang"},
	}, true); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := readFeederFile(t, feeder)
	if expect, have := "vim", got.Editor; expect != have {
		t.Errorf("editor: expected %q, got %q", expect, have)
	}
	if expect, have := "clang", got.Toolchain["CC"]; expect != have {
		t.Errorf("toolchain.CC: expected %q, got %q", expect, have)
	}
}

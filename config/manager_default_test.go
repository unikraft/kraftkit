// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").

package config

import "testing"

type TestConfig struct {
	Port int `yaml:"port" default:"8080"`
}

type TestConfigNoDefault struct {
	Port int `yaml:"port"`
}

func TestDefaultReturnsYAMLTaggedDefault(t *testing.T) {
	got := Default[TestConfig]("port")
	if got != "8080" {
		t.Fatalf("expected default %q, got %q", "8080", got)
	}
}

func TestDefaultReturnsEmptyStringWhenNoDefaultTag(t *testing.T) {
	got := Default[TestConfigNoDefault]("port")
	if got != "" {
		t.Fatalf("expected empty default, got %q", got)
	}
}

func TestDefaultNested(t *testing.T) {
	type Nested struct {
		Inner string `yaml:"inner" default:"val"`
	}

	type Config struct {
		Nested Nested `yaml:"nested"`
	}

	got := Default[Config]("nested.inner")
	if got != "val" {
		t.Fatalf("expected nested default %q, got %q", "val", got)
	}

	missing := Default[Config]("nested.unknown")
	if missing != "" {
		t.Fatalf("expected empty default for missing nested key, got %q", missing)
	}
}

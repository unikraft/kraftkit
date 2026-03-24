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

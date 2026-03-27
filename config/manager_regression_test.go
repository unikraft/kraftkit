// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").

package config

import (
	"reflect"
	"testing"
)

type regressionYAMLOnly struct {
	Port int `yaml:"port" default:"8080"`
}

type regressionJSONOnly struct {
	Port int `json:"port" default:"8080"`
}

type regressionNested struct {
	Server regressionNestedServer `yaml:"server"`
}

type regressionNestedServer struct {
	Port int `yaml:"port" default:"9090"`
}

type regressionNoDefault struct {
	Port int `yaml:"port"`
}

func TestRegressionDefault_YAMLTagResolution(t *testing.T) {
	// Expected behavior: Default resolves defaults by YAML key names.
	// Before the YAML-tag fix, this fails because lookup used JSON tags.
	// Bug exposed: wrong tag namespace in findConfigDefault.
	got := Default[regressionYAMLOnly]("port")
	if got != "8080" {
		t.Fatalf("expected default %q, got %q", "8080", got)
	}
}

func TestRegressionDefault_JSONTagFallback(t *testing.T) {
	// Expected behavior: JSON tags should not be required or preferred for config defaults.
	// In the broken implementation, JSON tag lookup can create an incorrect dependency.
	// Bug exposed: behavior tied to json tags instead of yaml tags.
	got := Default[regressionJSONOnly]("port")
	if got != "" {
		t.Fatalf("expected empty default for YAML key lookup on json-only tag, got %q", got)
	}
}

func TestRegressionDefault_NestedYAMLTraversal(t *testing.T) {
	// Expected behavior: recursive traversal resolves nested YAML-tagged fields.
	// Before the fixes, this can fail from broken tag matching and/or broken root reflection.
	// Bug exposed: recursive path resolution for nested defaults.
	got := Default[regressionNested]("server.port")
	if got != "9090" {
		t.Fatalf("expected nested default %q, got %q", "9090", got)
	}
}

func TestRegressionFindConfigDefault_ReflectionTraversal(t *testing.T) {
	// Expected behavior: helper visits struct fields when started with a real pointer to C.
	// This test is a debug-style isolation check for traversal independent of Default().
	// Bug exposed: reflection root must be compatible with struct traversal.
	_, found, def, _, err := findConfigDefault[regressionYAMLOnly](
		"port",
		"",
		"",
		reflect.ValueOf(new(regressionYAMLOnly)),
	)
	if err != nil {
		t.Fatalf("expected traversal to succeed, got error: %v", err)
	}
	if found != "port" {
		t.Fatalf("expected found key %q, got %q", "port", found)
	}
	if def != "8080" {
		t.Fatalf("expected discovered default %q, got %q", "8080", def)
	}
}

func TestRegressionDefault_MissingDefaultReturnsEmpty(t *testing.T) {
	// Expected behavior: fields without a default tag return empty string.
	// This guards against regressions while fixing tag resolution and traversal.
	// Bug exposed: incorrect non-empty fallback behavior.
	got := Default[regressionNoDefault]("port")
	if got != "" {
		t.Fatalf("expected empty default, got %q", got)
	}
}

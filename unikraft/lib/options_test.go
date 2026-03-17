// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package lib

import (
	"testing"

	"kraftkit.sh/make"
)

func TestWithIsInternal(t *testing.T) {
	for _, internal := range []bool{true, false} {
		lc := &LibraryConfig{}
		opt := WithIsInternal(internal)
		if err := opt(lc); err != nil {
			t.Fatalf("WithIsInternal(%v) returned error: %v", internal, err)
		}
		if lc.internal != internal {
			t.Errorf("WithIsInternal(%v) set internal = %v, want %v", internal, lc.internal, internal)
		}
	}
}

func TestWithName(t *testing.T) {
	lc := &LibraryConfig{}
	if err := WithName("mylib")(lc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lc.name != "mylib" {
		t.Errorf("WithName set name = %q, want %q", lc.name, "mylib")
	}
}

func TestWithSource(t *testing.T) {
	lc := &LibraryConfig{}
	if err := WithSource("https://example.com/lib.tar.gz")(lc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lc.source != "https://example.com/lib.tar.gz" {
		t.Errorf("WithSource set source = %q, want %q", lc.source, "https://example.com/lib.tar.gz")
	}
}

func TestWithVersion(t *testing.T) {
	lc := &LibraryConfig{}
	if err := WithVersion("1.2.3")(lc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lc.version != "1.2.3" {
		t.Errorf("WithVersion set version = %q, want %q", lc.version, "1.2.3")
	}
}

func TestWithCFlags(t *testing.T) {
	cv := []*make.ConditionalValue{{Value: "-O2"}}
	lc := &LibraryConfig{}
	if err := WithCFlags(cv)(lc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lc.cflags) != 1 || lc.cflags[0].Value != "-O2" {
		t.Errorf("WithCFlags did not set cflags correctly")
	}
}

func TestWithLicense(t *testing.T) {
	lc := &LibraryConfig{}
	if err := WithLicense("BSD-3-Clause")(lc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lc.license != "BSD-3-Clause" {
		t.Errorf("WithLicense set license = %q, want %q", lc.license, "BSD-3-Clause")
	}
}

func TestWithCompiler(t *testing.T) {
	lc := &LibraryConfig{}
	if err := WithCompiler("gcc")(lc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lc.compiler != "gcc" {
		t.Errorf("WithCompiler set compiler = %q, want %q", lc.compiler, "gcc")
	}
}

func TestWithCompileDate(t *testing.T) {
	lc := &LibraryConfig{}
	if err := WithCompileDate("2024-01-01")(lc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lc.compileDate != "2024-01-01" {
		t.Errorf("WithCompileDate set compileDate = %q, want %q", lc.compileDate, "2024-01-01")
	}
}

func TestWithCompiledBy(t *testing.T) {
	lc := &LibraryConfig{}
	if err := WithCompiledBy("Alice")(lc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lc.compiledBy != "Alice" {
		t.Errorf("WithCompiledBy set compiledBy = %q, want %q", lc.compiledBy, "Alice")
	}
}

func TestWithCompiledByAssoc(t *testing.T) {
	lc := &LibraryConfig{}
	if err := WithCompiledByAssoc("Unikraft GmbH")(lc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lc.compiledByAssoc != "Unikraft GmbH" {
		t.Errorf("WithCompiledByAssoc set compiledByAssoc = %q, want %q", lc.compiledByAssoc, "Unikraft GmbH")
	}
}

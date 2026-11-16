// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package selection

import "testing"

type testOption string

func (o testOption) String() string { return string(o) }

func TestSelectNoOptions(t *testing.T) {
	result, err := Select[testOption]("Select a package")
	if result != nil {
		t.Fatalf("expected no selection, got %v", result)
	}
	if err == nil || err.Error() != "no choices provided" {
		t.Fatalf("expected no choices provided, got %v", err)
	}
}

func TestSelectSingleOption(t *testing.T) {
	option := testOption("package")
	result, err := Select("Select a package", option)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || *result != option {
		t.Fatalf("expected %q, got %v", option, result)
	}
}

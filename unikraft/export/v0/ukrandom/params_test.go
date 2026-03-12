// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ukrandom

import (
	"fmt"
	"strings"
	"testing"
)

func TestExportedParams(t *testing.T) {
	params := ExportedParams()
	if len(params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(params))
	}
	if got := params[0].Name(); got != "random.seed" {
		t.Errorf("got %q, want %q", got, "random.seed")
	}
}

func TestNewRandomSeed_Length(t *testing.T) {
	rng := NewRandomSeed()
	if len(rng) != 8 {
		t.Errorf("expected length 8, got %d", len(rng))
	}
}

func TestRandomSeed_String(t *testing.T) {
	rng := RandomSeed{0, 1, 2, 3, 4, 5, 6, 7}
	got := rng.String()

	if !strings.HasPrefix(got, "[ ") {
		t.Errorf("expected prefix \"[ \", got %q", got)
	}
	if !strings.HasSuffix(got, "]") {
		t.Errorf("expected suffix \"]\", got %q", got)
	}
	for i, v := range rng {
		hex := fmt.Sprintf("0x%04x", v)
		if !strings.Contains(got, hex) {
			t.Errorf("index %d: expected %q in %q", i, hex, got)
		}
	}
}

func TestRandomSeed_String_AllZero(t *testing.T) {
	rng := RandomSeed{}
	got := rng.String()
	want := "[ 0x0000 0x0000 0x0000 0x0000 0x0000 0x0000 0x0000 0x0000 ]"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

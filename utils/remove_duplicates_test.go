// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"testing"
)

func TestRemoveDuplicates_empty(t *testing.T) {
	result := RemoveDuplicates([]int{})
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestRemoveDuplicates_singleElement(t *testing.T) {
	result := RemoveDuplicates([]int{42})
	if len(result) != 1 || result[0] != 42 {
		t.Errorf("expected [42], got %v", result)
	}
}

func TestRemoveDuplicates_noDuplicates(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	result := RemoveDuplicates(input)
	if len(result) != 5 {
		t.Errorf("expected 5 elements, got %d: %v", len(result), result)
	}
	for i, v := range []int{1, 2, 3, 4, 5} {
		if result[i] != v {
			t.Errorf("expected result[%d]=%d, got %d", i, v, result[i])
		}
	}
}

func TestRemoveDuplicates_allSame(t *testing.T) {
	result := RemoveDuplicates([]int{7, 7, 7, 7})
	if len(result) != 1 || result[0] != 7 {
		t.Errorf("expected [7], got %v", result)
	}
}

func TestRemoveDuplicates_mixed(t *testing.T) {
	result := RemoveDuplicates([]int{1, 1, 2, 3, 3, 3, 4, 5, 5})
	expected := []int{1, 2, 3, 4, 5}
	if len(result) != len(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("expected result[%d]=%d, got %d", i, v, result[i])
		}
	}
}

func TestRemoveDuplicates_strings(t *testing.T) {
	result := RemoveDuplicates([]string{"a", "a", "b", "c", "c"})
	expected := []string{"a", "b", "c"}
	if len(result) != len(expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("expected result[%d]=%s, got %s", i, v, result[i])
		}
	}
}

func TestRemoveDuplicates_twoElements_same(t *testing.T) {
	result := RemoveDuplicates([]int{5, 5})
	if len(result) != 1 || result[0] != 5 {
		t.Errorf("expected [5], got %v", result)
	}
}

func TestRemoveDuplicates_twoElements_different(t *testing.T) {
	result := RemoveDuplicates([]int{3, 7})
	if len(result) != 2 || result[0] != 3 || result[1] != 7 {
		t.Errorf("expected [3 7], got %v", result)
	}
}

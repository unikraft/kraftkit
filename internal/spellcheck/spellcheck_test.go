// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package spellcheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidate_KnownOption(t *testing.T) {
	// A known option should produce no results
	known := []string{"LIBVFSCORE", "LIBUKDEBUG", "LIBUKALLOC"}
	results := Validate([]string{"CONFIG_LIBVFSCORE=y"}, known)
	if len(results) != 0 {
		t.Errorf("expected 0 results for known option, got %d", len(results))
	}
}

func TestValidate_CloseTypo(t *testing.T) {
	// LIBWFSCORE is a close typo for LIBVFSCORE (1 char difference)
	known := []string{"LIBVFSCORE", "LIBUKDEBUG", "LIBUKALLOC"}
	results := Validate([]string{"CONFIG_LIBWFSCORE=y"}, known)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Option != "LIBWFSCORE" {
		t.Errorf("expected option LIBWFSCORE, got %s", results[0].Option)
	}
	if results[0].Suggestion != "LIBVFSCORE" {
		t.Errorf("expected suggestion LIBVFSCORE, got %s", results[0].Suggestion)
	}
	if results[0].Similarity < 0.7 {
		t.Errorf("expected similarity >= 0.7, got %f", results[0].Similarity)
	}
}

func TestValidate_FarOffOption(t *testing.T) {
	// A completely unrelated string should not produce a suggestion
	known := []string{"LIBVFSCORE", "LIBUKDEBUG", "LIBUKALLOC"}
	results := Validate([]string{"XYZABC123"}, known)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Suggestion != "" {
		t.Errorf("expected no suggestion for far-off option, got %s", results[0].Suggestion)
	}
}

func TestValidate_WithoutPrefix(t *testing.T) {
	// Options provided without CONFIG_ prefix should also work
	known := []string{"LIBVFSCORE", "LIBUKDEBUG"}
	results := Validate([]string{"LIBWFSCORE=y"}, known)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Suggestion != "LIBVFSCORE" {
		t.Errorf("expected suggestion LIBVFSCORE, got %s", results[0].Suggestion)
	}
}

func TestValidate_EmptyInputs(t *testing.T) {
	// Empty user options should produce no results
	results := Validate([]string{}, []string{"LIBVFSCORE"})
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(results))
	}

	// Empty known configs should mark all options as unknown
	results = Validate([]string{"CONFIG_LIBVFSCORE=y"}, []string{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result for empty known configs, got %d", len(results))
	}
	if results[0].Suggestion != "" {
		t.Errorf("expected no suggestion with empty known configs, got %s", results[0].Suggestion)
	}
}

func TestValidate_MultipleOptions(t *testing.T) {
	// Mix of known and unknown options
	known := []string{"LIBVFSCORE", "LIBUKDEBUG", "LIBUKALLOC"}
	results := Validate([]string{
		"CONFIG_LIBVFSCORE=y",  // known
		"CONFIG_LIBWFSCORE=y",  // typo
		"CONFIG_LIBUKDEBUG=y",  // known
		"CONFIG_XYZNOTEXIST=n", // unknown, no close match
	}, known)
	if len(results) != 2 {
		t.Fatalf("expected 2 unknown results, got %d", len(results))
	}
}

func TestValidate_CustomThreshold(t *testing.T) {
	// With a very high threshold, close matches should not qualify
	known := []string{"LIBVFSCORE", "LIBUKDEBUG"}
	results := Validate(
		[]string{"CONFIG_LIBWFSCORE=y"},
		known,
		WithSimilarityThreshold(0.99),
	)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Suggestion != "" {
		t.Errorf("expected no suggestion with 0.99 threshold, got %s", results[0].Suggestion)
	}
}

// writeTempConfig creates a temporary .config file with the given content
// and returns its path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".config")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

func TestAllKConfigSymbols_ActiveEntries(t *testing.T) {
	path := writeTempConfig(t, "CONFIG_LIBVFSCORE=y\nCONFIG_LIBUKDEBUG=y\nCONFIG_LWIP_TCP_SND_BUF=4096\n")

	symbols, err := AllKConfigSymbols(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symbols) != 3 {
		t.Fatalf("expected 3 symbols, got %d", len(symbols))
	}

	expected := map[string]bool{
		"CONFIG_LIBVFSCORE":       true,
		"CONFIG_LIBUKDEBUG":       true,
		"CONFIG_LWIP_TCP_SND_BUF": true,
	}
	for _, s := range symbols {
		if !expected[s] {
			t.Errorf("unexpected symbol: %s", s)
		}
	}
}

func TestAllKConfigSymbols_CommentedOutEntries(t *testing.T) {
	path := writeTempConfig(t, "# CONFIG_LIBVFSCORE is not set\n# CONFIG_LIBUKDEBUG is not set\n")

	symbols, err := AllKConfigSymbols(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d", len(symbols))
	}

	expected := map[string]bool{
		"CONFIG_LIBVFSCORE": true,
		"CONFIG_LIBUKDEBUG": true,
	}
	for _, s := range symbols {
		if !expected[s] {
			t.Errorf("unexpected symbol: %s", s)
		}
	}
}

func TestAllKConfigSymbols_MixedContent(t *testing.T) {
	// Realistic .config with active, commented-out, blank lines, and header comments
	content := `#
# Automatically generated file; DO NOT EDIT.
# Unikraft Configuration
#
CONFIG_LIBVFSCORE=y
CONFIG_LIBUKDEBUG=y
# CONFIG_LIBUKALLOC is not set
CONFIG_LWIP_TCP_SND_BUF=4096

# Networking options
# CONFIG_LIBUKNETDEV is not set
`
	path := writeTempConfig(t, content)

	symbols, err := AllKConfigSymbols(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symbols) != 5 {
		t.Fatalf("expected 5 symbols, got %d: %v", len(symbols), symbols)
	}

	expected := map[string]bool{
		"CONFIG_LIBVFSCORE":       true,
		"CONFIG_LIBUKDEBUG":       true,
		"CONFIG_LIBUKALLOC":       true,
		"CONFIG_LWIP_TCP_SND_BUF": true,
		"CONFIG_LIBUKNETDEV":      true,
	}
	for _, s := range symbols {
		if !expected[s] {
			t.Errorf("unexpected symbol: %s", s)
		}
	}
}

func TestAllKConfigSymbols_DuplicateSymbols(t *testing.T) {
	// Same symbol appears as both active and commented-out
	content := "CONFIG_LIBVFSCORE=y\n# CONFIG_LIBVFSCORE is not set\nCONFIG_LIBVFSCORE=n\n"
	path := writeTempConfig(t, content)

	symbols, err := AllKConfigSymbols(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symbols) != 1 {
		t.Fatalf("expected 1 deduplicated symbol, got %d: %v", len(symbols), symbols)
	}
	if symbols[0] != "CONFIG_LIBVFSCORE" {
		t.Errorf("expected CONFIG_LIBVFSCORE, got %s", symbols[0])
	}
}

func TestAllKConfigSymbols_MalformedLines(t *testing.T) {
	// Lines without = or without CONFIG_ prefix should be skipped
	content := "CONFIG_LIBVFSCORE=y\nsome random text\n=nokey\n\nCONFIG_LIBUKDEBUG=y\n"
	path := writeTempConfig(t, content)

	symbols, err := AllKConfigSymbols(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "some random text" has no =, so skipped
	// "=nokey" has empty key before =, so skipped
	// Only the two valid CONFIG_ entries should be parsed
	if len(symbols) != 2 {
		t.Fatalf("expected 2 symbols, got %d: %v", len(symbols), symbols)
	}
}

func TestAllKConfigSymbols_EmptyFile(t *testing.T) {
	path := writeTempConfig(t, "")

	symbols, err := AllKConfigSymbols(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(symbols) != 0 {
		t.Errorf("expected 0 symbols for empty file, got %d", len(symbols))
	}
}

func TestAllKConfigSymbols_NonexistentFile(t *testing.T) {
	_, err := AllKConfigSymbols("/nonexistent/path/.config")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License.
package name

import (
	"regexp"
	"strings"
	"testing"
)

func TestNewRandomMachineName(t *testing.T) {
	for i := 0; i < 100; i++ {
		name := NewRandomMachineName(0)
		if name == "" {
			t.Errorf("iteration %d: expected non-empty string", i)
		}
		if !strings.Contains(name, "_") {
			t.Errorf("iteration %d: expected underscore in name, got %q", i, name)
		}
		parts := strings.Split(name, "_")
		if len(parts) != 2 {
			t.Errorf("iteration %d: expected two parts separated by underscore, got %v", i, parts)
		}
	}

	for i := 0; i < 100; i++ {
		name := NewRandomMachineName(1)
		if name == "" {
			t.Errorf("retry %d: expected non-empty string", i)
		}
		if !strings.Contains(name, "_") {
			t.Errorf("retry %d: expected underscore in name, got %q", i, name)
		}
		// Should end with a digit or letter
		if !regexp.MustCompile(`_[a-z]+[0-9]$`).MatchString(name) && !regexp.MustCompile(`_[a-z]+$`).MatchString(name) {
			t.Errorf("retry %d: expected name to end with a digit or letter, got %q", i, name)
		}
	}
}

func TestNewRandomMachineID(t *testing.T) {
	for i := 0; i < 100; i++ {
		id, err := NewRandomMachineID()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if len(id) != 64 {
			t.Errorf("iteration %d: expected 64 character id, got %d", i, len(id))
		}
		matched := regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(string(id))
		if !matched {
			t.Errorf("iteration %d: expected lowercase hex string, got %q", i, id)
		}

		// Validate truncated ID (12 chars)
		shortID := TruncateMachineID(id)
		if err := ValidateMachineID(shortID.String()); err != nil {
			t.Errorf("iteration %d: ValidateMachineID failed on truncated ID %q: %v", i, shortID, err)
		}
	}
}

func TestValidateMachineID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid lowercase hex (12 chars)", "abcdef123456", false},
		{"uppercase hex", "ABCDEF123456", true},
		{"too short", "abc123", true},
		{"too long", "abcdef1234567890", true},
		{"non-hex characters", "abcde!@#4567", true},
		{"empty string", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMachineID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMachineID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

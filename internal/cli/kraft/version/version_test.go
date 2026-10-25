// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package version

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/internal/version"
	"kraftkit.sh/iostreams"
)

func TestVersionNewCmdRegistration(t *testing.T) {
	cmd := NewCmd()
	if cmd == nil {
		t.Fatal("expected NewCmd to return non-nil command")
	}

	if cmd.Use != "version" {
		t.Errorf("unexpected cmd.Use: %q, expected %q", cmd.Use, "version")
	}

	// Verify alias 'v'
	hasAlias := false
	for _, alias := range cmd.Aliases {
		if alias == "v" {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		t.Errorf("expected cmd.Aliases to contain 'v', got %v", cmd.Aliases)
	}

	// Verify AnnotationHelpGroup
	if cmd.Annotations[cmdfactory.AnnotationHelpGroup] != "misc" {
		t.Errorf("expected AnnotationHelpGroup to be 'misc', got %q", cmd.Annotations[cmdfactory.AnnotationHelpGroup])
	}

	// Verify Args constraint: NoArgs must be configured
	if cmd.Args == nil {
		t.Fatal("expected cmd.Args constraint to be set (cobra.NoArgs)")
	}
}

func TestVersionArgs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			name:      "no args accepted",
			args:      []string{},
			expectErr: false,
		},
		{
			name:      "extra args rejected",
			args:      []string{"extra"},
			expectErr: true,
		},
		{
			name:      "multiple extra args rejected",
			args:      []string{"extra1", "extra2"},
			expectErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmd()
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if tc.expectErr && err == nil {
				t.Fatalf("expected error for args %v, got nil", tc.args)
			} else if !tc.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestVersionRun(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	ios := &iostreams.IOStreams{
		Out: w,
	}
	ctx := iostreams.WithIOStreams(context.Background(), ios)

	opts := &VersionOptions{}
	if err := opts.Run(ctx, nil); err != nil {
		t.Fatalf("opts.Run returned unexpected error: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close write pipe: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("failed to read from read pipe: %v", err)
	}
	_ = r.Close()

	actual := buf.String()
	if !strings.HasPrefix(actual, "kraft ") {
		t.Errorf("expected output to start with 'kraft ', got: %q", actual)
	}
	if !strings.Contains(actual, version.String()) {
		t.Errorf("expected output to contain version string %q, got: %q", version.String(), actual)
	}
}

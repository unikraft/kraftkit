// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package version

import (
	"bytes"
	"context"
	"fmt"
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

	if cmd.Short != "Show kraft version information" {
		t.Errorf("unexpected cmd.Short: %q", cmd.Short)
	}

	if cmd.Long != "Show kraft version information." {
		t.Errorf("unexpected cmd.Long: %q", cmd.Long)
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

	// Verify Args constraint: NoArgs
	if cmd.Args != nil {
		if err := cmd.Args(cmd, []string{}); err != nil {
			t.Errorf("expected 0 args to be accepted, got error: %v", err)
		}
		if err := cmd.Args(cmd, []string{"extra-arg"}); err == nil {
			t.Errorf("expected error when passing extra args to version command, got nil")
		}
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

	expected := fmt.Sprintf("kraft %s", version.String())
	actual := buf.String()

	if actual != expected {
		t.Errorf("unexpected output from version.Run:\nexpected: %q\ngot:      %q", expected, actual)
	}
}

func TestInternalVersionPackage(t *testing.T) {
	str := version.String()
	if !strings.Contains(str, version.Version()) {
		t.Errorf("expected version.String() to contain version %q", version.Version())
	}
	if !strings.Contains(str, version.Commit()) {
		t.Errorf("expected version.String() to contain commit %q", version.Commit())
	}
	if !strings.Contains(str, version.BuildTime()) {
		t.Errorf("expected version.String() to contain buildTime %q", version.BuildTime())
	}

	ua := version.UserAgent()
	if ua == "" {
		t.Errorf("expected non-empty UserAgent")
	}
}

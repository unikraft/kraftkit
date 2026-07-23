// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").

package initrd_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"kraftkit.sh/initrd"
)

func TestNewFromFileBuildEmptyOutputReturnsSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "initrd.cpio")
	// Standard CPIO magic header "070701"
	if err := os.WriteFile(src, []byte("07070100000000"), 0o644); err != nil {
		t.Fatal(err)
	}

	ird, err := initrd.NewFromFile(context.Background(), src)
	if err != nil {
		t.Fatalf("NewFromFile: %v", err)
	}

	out, err := ird.Build(context.Background())
	if err != nil {
		t.Fatalf("Build with empty output: %v", err)
	}
	if out != src {
		t.Fatalf("Build() = %q, want source path %q", out, src)
	}
}

func TestNewFromFileSameSourceAndOutputAllowedForCpio(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "initrd.cpio")
	if err := os.WriteFile(src, []byte("07070100000000"), 0o644); err != nil {
		t.Fatal(err)
	}

	ird, err := initrd.NewFromFile(context.Background(), src, initrd.WithOutput(src))
	if err != nil {
		t.Fatalf("expected NewFromFile to succeed when source and output match for CPIO, got: %v", err)
	}

	out, err := ird.Build(context.Background())
	if err != nil {
		t.Fatalf("expected Build to succeed when source and output match for CPIO, got: %v", err)
	}
	if out != src {
		t.Fatalf("Build() = %q, want source path %q", out, src)
	}
}

func TestNewFromFileSameSourceAndOutputRejectedForNonArchive(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "script.sh")
	// Regular non-CPIO file
	if err := os.WriteFile(src, []byte("#!/bin/sh\necho hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	ird, err := initrd.NewFromFile(context.Background(), src, initrd.WithOutput(src))
	if err != nil {
		t.Fatalf("unexpected error from NewFromFile: %v", err)
	}

	_, err = ird.Build(context.Background())
	if err == nil {
		t.Fatal("expected error from Build() when non-archive source and output are the same path")
	}
}

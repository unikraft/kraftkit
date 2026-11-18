// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package unset

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestUnsetNewCmd(t *testing.T) {
	cmd := NewCmd()
	if cmd == nil {
		t.Fatal("expected NewCmd to return non-nil command")
	}

	saveFlag := cmd.Flag("save")
	if saveFlag == nil {
		t.Fatal("expected --save flag to be registered")
	}
	if saveFlag.Shorthand != "s" {
		t.Errorf("expected --save shorthand to be 's', got %q", saveFlag.Shorthand)
	}

	for _, flagName := range []string{"arch", "kraftfile", "plat", "target", "workdir"} {
		if cmd.Flag(flagName) == nil {
			t.Errorf("expected --%s flag to be registered", flagName)
		}
	}
}

func TestUnsetValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		expectErr error
	}{
		{
			name:      "no options to unset",
			args:      []string{},
			expectErr: ErrNoOptionsToUnset,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := UnsetOptions{}
			err := opts.Run(context.Background(), tc.args)
			if err == nil || !errors.Is(err, tc.expectErr) {
				t.Fatalf("expected error wrapping %v, got: %v", tc.expectErr, err)
			}
		})
	}
}

func TestUnsetNoMatchingTargets(t *testing.T) {
	tmpDir := t.TempDir()
	kraftfilePath := filepath.Join(tmpDir, "Kraftfile")
	err := os.WriteFile(kraftfilePath, []byte(`spec: v0.6
name: test-app
targets:
  - name: my-target
    architecture: x86_64
    platform: qemu
`), 0o644)
	if err != nil {
		t.Fatalf("failed to write Kraftfile: %v", err)
	}

	opts := UnsetOptions{
		Workdir: tmpDir,
		Target:  "non-existent-target",
		Save:    true,
	}

	err = opts.Run(context.Background(), []string{"CONFIG_LIBUKDEBUG"})
	if !errors.Is(err, ErrNoMatchingTargets) {
		t.Fatalf("expected ErrNoMatchingTargets, got: %v", err)
	}
}

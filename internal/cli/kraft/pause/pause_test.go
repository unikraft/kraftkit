// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pause

import (
	"context"
	"strings"
	"testing"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
)

func TestPauseNewCmdRegistration(t *testing.T) {
	cmd := NewCmd()
	if cmd == nil {
		t.Fatal("expected NewCmd to return non-nil command")
	}

	if !strings.HasPrefix(cmd.Use, "pause") {
		t.Errorf("unexpected cmd.Use: %q", cmd.Use)
	}

	if cmd.Annotations[cmdfactory.AnnotationHelpGroup] != "run" {
		t.Errorf("expected AnnotationHelpGroup to be 'run', got %q", cmd.Annotations[cmdfactory.AnnotationHelpGroup])
	}

	// Verify flags exist
	if cmd.Flag("all") == nil {
		t.Fatal("expected --all flag to be registered")
	}
	if cmd.Flag("plat") == nil {
		t.Fatal("expected --plat flag to be registered")
	}
}

func TestPausePreValidation(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		expectErr string
	}{
		{
			name:      "no args and no all flag",
			args:      []string{},
			expectErr: "please supply a machine ID or name or use the --all flag",
		},
		{
			name:      "all flag with no machine args",
			args:      []string{"--all"},
			expectErr: "machine(s) not found",
		},
		{
			name:      "machine arg without all flag",
			args:      []string{"my-unikernel"},
			expectErr: "machine(s) not found",
		},
		{
			name:      "machine args with explicit platform flag",
			args:      []string{"--plat", "all", "machine-1"},
			expectErr: "machine(s) not found",
		},
		{
			name:      "machine args with platform shorthand",
			args:      []string{"-p", "all", "machine-1"},
			expectErr: "machine(s) not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
				Config: &config.KraftKit{RuntimeDir: t.TempDir()},
			})
			cmd := NewCmd()
			cmd.SetArgs(tc.args)
			err := cmd.ExecuteContext(ctx)
			if tc.expectErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.expectErr) {
					t.Fatalf("expected error containing %q, got: %v", tc.expectErr, err)
				}
			} else if err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
		})
	}
}

func TestPauseRunUnknownPlatform(t *testing.T) {
	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{RuntimeDir: t.TempDir()},
	})
	opts := PauseOptions{
		Platform: "unknown-plat-xyz",
	}
	err := opts.Run(ctx, []string{"my-vm"})
	if err == nil {
		t.Fatal("expected error for unknown platform driver, got nil")
	}
	expected := "unknown platform driver: unknown-plat-xyz"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error containing %q, got: %v", expected, err)
	}
}

func TestPauseRunMachineNotFound(t *testing.T) {
	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{RuntimeDir: t.TempDir()},
	})
	opts := PauseOptions{
		Platform: "all",
	}
	err := opts.Run(ctx, []string{"nonexistent-machine-xyz"})
	if err == nil {
		t.Fatal("expected error for nonexistent machine, got nil")
	}
	expected := "machine(s) not found"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error containing %q, got: %v", expected, err)
	}
}

func TestPauseNilOptions(t *testing.T) {
	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{RuntimeDir: t.TempDir()},
	})
	err := Pause(ctx, nil)
	if err == nil {
		t.Fatal("expected error when running Pause with empty args and nil opts, got nil")
	}
	expectedErr := "please supply a machine ID or name or use the --all flag"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
	}
}

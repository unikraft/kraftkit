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
)

func TestPauseNewCmdRegistration(t *testing.T) {
	cmd := NewCmd()
	if cmd == nil {
		t.Fatal("expected NewCmd to return non-nil command")
	}

	expectedUse := "pause [FLAGS] MACHINE [MACHINE [...]]"
	if cmd.Use != expectedUse {
		t.Errorf("unexpected cmd.Use: %q, expected %q", cmd.Use, expectedUse)
	}

	expectedShort := "Pause one or more running unikernels"
	if cmd.Short != expectedShort {
		t.Errorf("unexpected cmd.Short: %q, expected %q", cmd.Short, expectedShort)
	}

	if cmd.Annotations[cmdfactory.AnnotationHelpGroup] != "run" {
		t.Errorf("expected AnnotationHelpGroup to be 'run', got %q", cmd.Annotations[cmdfactory.AnnotationHelpGroup])
	}

	// Verify --all flag
	allFlag := cmd.Flag("all")
	if allFlag == nil {
		t.Fatal("expected --all flag to be registered")
	}
	if allFlag.DefValue != "false" {
		t.Errorf("expected --all default value to be 'false', got %q", allFlag.DefValue)
	}

	// Verify --plat flag
	platFlag := cmd.Flag("plat")
	if platFlag == nil {
		t.Fatal("expected --plat flag to be registered")
	}
	if platFlag.Shorthand != "p" {
		t.Errorf("expected shorthand 'p' for --plat flag, got %q", platFlag.Shorthand)
	}
	if platFlag.DefValue != "all" {
		t.Errorf("expected --plat default value to be 'all', got %q", platFlag.DefValue)
	}
}

func TestPausePreValidation(t *testing.T) {
	tests := []struct {
		name           string
		opts           PauseOptions
		platFlagValue  string
		args           []string
		expectErr      string
		expectPlatform string
	}{
		{
			name: "no args and no all flag",
			opts: PauseOptions{
				All: false,
			},
			args:      []string{},
			expectErr: "please supply a machine ID or name or use the --all flag",
		},
		{
			name: "all flag with no args default platform",
			opts: PauseOptions{
				All: true,
			},
			args:           []string{},
			expectPlatform: "all",
		},
		{
			name: "single machine arg",
			opts: PauseOptions{
				All: false,
			},
			args:           []string{"my-unikernel"},
			expectPlatform: "all",
		},
		{
			name: "multiple machine args with explicit platform",
			opts: PauseOptions{
				All: false,
			},
			platFlagValue:  "qemu",
			args:           []string{"machine-1", "machine-2"},
			expectPlatform: "qemu",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmd()
			if tc.platFlagValue != "" {
				if err := cmd.Flags().Set("plat", tc.platFlagValue); err != nil {
					t.Fatalf("failed to set plat flag: %v", err)
				}
			}

			err := tc.opts.Pre(cmd, tc.args)
			if tc.expectErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.expectErr)
				}
				if !strings.Contains(err.Error(), tc.expectErr) {
					t.Errorf("expected error containing %q, got %q", tc.expectErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error from Pre: %v", err)
			}

			if tc.opts.Platform != tc.expectPlatform {
				t.Errorf("expected platform %q, got %q", tc.expectPlatform, tc.opts.Platform)
			}
		})
	}
}

func TestPauseNilOptions(t *testing.T) {
	ctx := context.Background()
	err := Pause(ctx, nil)
	if err == nil {
		t.Fatal("expected error when running Pause with empty args and nil opts, got nil")
	}
	expectedErr := "please supply a machine ID or name or use the --all flag"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error containing %q, got %q", expectedErr, err.Error())
	}
}

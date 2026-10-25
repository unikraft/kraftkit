// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package net

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"kraftkit.sh/cmdfactory"
)

func TestNetNewCmdRegistration(t *testing.T) {
	cmd := NewCmd()
	if cmd == nil {
		t.Fatal("expected NewCmd to return non-nil command")
	}

	if cmd.Use != "net SUBCOMMAND" {
		t.Errorf("unexpected cmd.Use: %q, expected %q", cmd.Use, "net SUBCOMMAND")
	}

	// Verify aliases
	hasAlias := false
	for _, alias := range cmd.Aliases {
		if alias == "network" {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		t.Errorf("expected cmd.Aliases to contain 'network', got %v", cmd.Aliases)
	}

	// Verify annotations
	if cmd.Annotations[cmdfactory.AnnotationHelpGroup] != "net" {
		t.Errorf("expected AnnotationHelpGroup to be 'net', got %q", cmd.Annotations[cmdfactory.AnnotationHelpGroup])
	}
	if cmd.Annotations[cmdfactory.AnnotationHelpHidden] != "true" {
		t.Errorf("expected AnnotationHelpHidden to be 'true', got %q", cmd.Annotations[cmdfactory.AnnotationHelpHidden])
	}

	// Verify subcommands
	expectedSubcommands := []string{
		"create",
		"down",
		"inspect",
		"list",
		"remove",
		"up",
	}

	subcmdMap := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subcmdMap[sub.Name()] = true
		// Verify that subcommands inherit the persistent --driver flag
		if sub.Flag("driver") == nil {
			t.Errorf("expected subcommand %q to inherit persistent flag 'driver'", sub.Name())
		}
	}

	for _, expected := range expectedSubcommands {
		if !subcmdMap[expected] {
			t.Errorf("expected subcommand %q to be registered on net command", expected)
		}
	}

	// Verify driver flag on root command
	flag := cmd.Flag("driver")
	if flag == nil {
		t.Fatal("expected --driver flag to be registered")
	}
	if flag.Shorthand != "d" {
		t.Errorf("expected shorthand 'd' for --driver flag, got %q", flag.Shorthand)
	}
	if flag.DefValue != "bridge" {
		t.Errorf("expected default driver value 'bridge', got %q", flag.DefValue)
	}
}

func TestNetPreValidation(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		expectErr string
	}{
		{
			name:      "unsupported driver via flags",
			args:      []string{"--driver", "unsupported-driver-xyz"},
			expectErr: "unsupported network driver strategy: unsupported-driver-xyz",
		},
		{
			name:      "unsupported driver shorthand",
			args:      []string{"-d", "unsupported-driver-xyz"},
			expectErr: "unsupported network driver strategy: unsupported-driver-xyz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmd()
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if tc.expectErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.expectErr) {
					t.Fatalf("expected error containing %q, got: %v", tc.expectErr, err)
				}
			} else if err != nil && !errors.Is(err, pflag.ErrHelp) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestNetRun(t *testing.T) {
	opts := NetOptions{
		Driver: "bridge",
	}

	err := opts.Run(context.Background(), nil)
	if !errors.Is(err, pflag.ErrHelp) {
		t.Fatalf("expected Run to return pflag.ErrHelp, got: %v", err)
	}
}

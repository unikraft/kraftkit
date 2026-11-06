// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package events

import (
	"context"
	"strings"
	"testing"
	"time"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
)

func TestEventsNewCmdRegistration(t *testing.T) {
	cmd := NewCmd()
	if cmd == nil {
		t.Fatal("expected NewCmd to return non-nil command")
	}

	if !strings.HasPrefix(cmd.Use, "events") {
		t.Errorf("unexpected cmd.Use: %q", cmd.Use)
	}

	// Verify alias 'event'
	hasAlias := false
	for _, alias := range cmd.Aliases {
		if alias == "event" {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		t.Errorf("expected cmd.Aliases to contain 'event', got %v", cmd.Aliases)
	}

	// Verify AnnotationHelpGroup
	if cmd.Annotations[cmdfactory.AnnotationHelpGroup] != "run" {
		t.Errorf("expected AnnotationHelpGroup to be 'run', got %q", cmd.Annotations[cmdfactory.AnnotationHelpGroup])
	}

	// Verify hidden attributes
	if !cmd.Hidden {
		t.Error("expected events command to be marked Hidden")
	}
	if cmd.Annotations[cmdfactory.AnnotationHelpHidden] != "true" {
		t.Errorf("expected AnnotationHelpHidden to be 'true', got %q", cmd.Annotations[cmdfactory.AnnotationHelpHidden])
	}

	// Verify flags registered
	for _, flag := range []string{
		"poll-granularity",
		"quit-together",
		"plat",
	} {
		if cmd.Flag(flag) == nil {
			t.Fatalf("expected --%s flag to be registered", flag)
		}
	}
}

func TestEventsArgs(t *testing.T) {
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
			name:      "single machine arg accepted",
			args:      []string{"my-vm-id"},
			expectErr: false,
		},
		{
			name:      "multiple args rejected",
			args:      []string{"vm1", "vm2"},
			expectErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewCmd()
			cmd.SetArgs(tc.args)
			err := cmd.Args(cmd, tc.args)
			if tc.expectErr && err == nil {
				t.Fatalf("expected error for args %v, got nil", tc.args)
			} else if !tc.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestEventsPrePlatformResolution(t *testing.T) {
	cmd := NewCmd()
	opts := &EventOptions{
		Granularity:  time.Second,
		QuitTogether: true,
	}

	cmd.SetArgs([]string{"--plat", "qemu"})
	if err := cmd.ParseFlags([]string{"--plat", "qemu"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := opts.Pre(cmd, nil); err != nil {
		t.Fatalf("opts.Pre returned unexpected error: %v", err)
	}

	if opts.platform != "qemu" {
		t.Errorf("expected platform to be 'qemu', got %q", opts.platform)
	}
}

func TestEventsRunUnknownPlatform(t *testing.T) {
	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})

	opts := &EventOptions{
		platform: "unknown-plat-xyz",
	}

	err := opts.Run(ctx, nil)
	if err == nil {
		t.Fatal("expected error for unknown platform driver, got nil")
	}

	expected := "unknown platform driver: unknown-plat-xyz"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error containing %q, got: %v", expected, err)
	}
}

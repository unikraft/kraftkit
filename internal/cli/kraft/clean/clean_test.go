// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package clean

import (
	"context"
	"strings"
	"testing"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/packmanager"
)

func initTestUmbrella(t *testing.T) {
	origUmbrella := packmanager.UmbrellaInstance
	packmanager.UmbrellaInstance = &packmanager.UmbrellaManager{}
	t.Cleanup(func() {
		packmanager.UmbrellaInstance = origUmbrella
	})
}

func TestCleanNewCmdRegistration(t *testing.T) {
	cmd := NewCmd()
	if cmd == nil {
		t.Fatal("expected NewCmd to return non-nil command")
	}

	if !strings.HasPrefix(cmd.Use, "clean") {
		t.Errorf("unexpected cmd.Use: %q", cmd.Use)
	}

	// Verify alias 'cln'
	hasAlias := false
	for _, alias := range cmd.Aliases {
		if alias == "cln" {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		t.Errorf("expected cmd.Aliases to contain 'cln', got %v", cmd.Aliases)
	}

	// Verify AnnotationHelpGroup
	if cmd.Annotations[cmdfactory.AnnotationHelpGroup] != "build" {
		t.Errorf("expected AnnotationHelpGroup to be 'build', got %q", cmd.Annotations[cmdfactory.AnnotationHelpGroup])
	}

	// Verify flags registered
	for _, flag := range []string{"arch", "kraftfile", "plat", "target", "proper"} {
		if cmd.Flag(flag) == nil {
			t.Fatalf("expected --%s flag to be registered", flag)
		}
	}
}

func TestCleanArgs(t *testing.T) {
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
			name:      "single dir arg accepted",
			args:      []string{t.TempDir()},
			expectErr: false,
		},
		{
			name:      "multiple args rejected",
			args:      []string{"dir1", "dir2"},
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

func TestCleanPre(t *testing.T) {
	initTestUmbrella(t)

	cmd := NewCmd()
	opts := &CleanOptions{
		Platform: "qemu",
	}

	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})
	cmd.SetContext(ctx)

	if err := opts.Pre(cmd, nil); err != nil {
		t.Fatalf("opts.Pre returned unexpected error: %v", err)
	}

	if opts.Platform != "qemu" {
		t.Errorf("expected Platform to be 'qemu', got %q", opts.Platform)
	}
}

func TestCleanDefensiveNilOptions(t *testing.T) {
	initTestUmbrella(t)

	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(ctx)
	if err != nil {
		t.Fatalf("failed to init umbrella manager: %v", err)
	}

	tmpDir := t.TempDir()
	err = Clean(ctx, nil, tmpDir)
	if err == nil {
		t.Fatal("expected error when cleaning empty directory without Kraftfile, got nil")
	}
}

func TestCleanRunMissingProject(t *testing.T) {
	initTestUmbrella(t)

	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})
	ctx, err := packmanager.WithDefaultUmbrellaManagerInContext(ctx)
	if err != nil {
		t.Fatalf("failed to init umbrella manager: %v", err)
	}

	tmpDir := t.TempDir()
	opts := &CleanOptions{}
	err = opts.Run(ctx, []string{tmpDir})
	if err == nil {
		t.Fatal("expected error running Clean on empty directory without project, got nil")
	}
}

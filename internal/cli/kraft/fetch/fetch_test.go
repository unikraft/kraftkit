// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package fetch

import (
	"context"
	"os"
	"path/filepath"
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

func TestFetchNewCmdRegistration(t *testing.T) {
	cmd := NewCmd()
	if cmd == nil {
		t.Fatal("expected NewCmd to return non-nil command")
	}

	if !strings.HasPrefix(cmd.Use, "fetch") {
		t.Errorf("unexpected cmd.Use: %q", cmd.Use)
	}

	// Verify alias 'ft'
	hasAlias := false
	for _, alias := range cmd.Aliases {
		if alias == "ft" {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		t.Errorf("expected cmd.Aliases to contain 'ft', got %v", cmd.Aliases)
	}

	// Verify AnnotationHelpGroup
	if cmd.Annotations[cmdfactory.AnnotationHelpGroup] != "build" {
		t.Errorf("expected AnnotationHelpGroup to be 'build', got %q", cmd.Annotations[cmdfactory.AnnotationHelpGroup])
	}

	// Verify hidden flags / annotations
	if !cmd.Hidden {
		t.Error("expected fetch command to be marked Hidden")
	}
	if cmd.Annotations[cmdfactory.AnnotationHelpHidden] != "true" {
		t.Errorf("expected AnnotationHelpHidden to be 'true', got %q", cmd.Annotations[cmdfactory.AnnotationHelpHidden])
	}

	// Verify flags registered
	for _, flag := range []string{
		"arch",
		"kraftfile",
		"no-cache",
		"no-configure",
		"no-pull",
		"plat",
		"target",
	} {
		if cmd.Flag(flag) == nil {
			t.Fatalf("expected --%s flag to be registered", flag)
		}
	}
}

func TestFetchArgs(t *testing.T) {
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

func TestFetchPreMissingKraftfile(t *testing.T) {
	initTestUmbrella(t)

	cmd := NewCmd()
	opts := &FetchOptions{}

	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})
	cmd.SetContext(ctx)

	tmpDir := t.TempDir()
	err := opts.Pre(cmd, []string{tmpDir})
	if err == nil {
		t.Fatal("expected error when running Pre on directory without Kraftfile, got nil")
	}

	expected := "cannot build project directory without a Kraftfile"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error containing %q, got: %v", expected, err)
	}
}

func TestFetchPreWithKraftfile(t *testing.T) {
	initTestUmbrella(t)

	tmpDir := t.TempDir()
	kraftfilePath := filepath.Join(tmpDir, "Kraftfile")
	kraftfileContent := "spec: v0.7\nname: testapp\ntargets:\n  - name: testtarget\n    plat: qemu\n    arch: x86_64\n"
	if err := os.WriteFile(kraftfilePath, []byte(kraftfileContent), 0o644); err != nil {
		t.Fatalf("failed to create Kraftfile: %v", err)
	}

	cmd := NewCmd()
	opts := &FetchOptions{
		Platform: "qemu",
	}

	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})
	cmd.SetContext(ctx)

	err := opts.Pre(cmd, []string{tmpDir})
	if err != nil {
		t.Fatalf("opts.Pre returned unexpected error: %v", err)
	}

	if opts.project == nil {
		t.Fatal("expected opts.project to be initialized")
	}

	if opts.Platform != "qemu" {
		t.Errorf("expected Platform to be 'qemu', got %q", opts.Platform)
	}
}

func TestFetchDefensiveNilProject(t *testing.T) {
	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})

	opts := &FetchOptions{}
	err := opts.Run(ctx, nil)
	if err == nil {
		t.Fatal("expected error when running Run with nil project, got nil")
	}

	expected := "project not initialized"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error containing %q, got: %v", expected, err)
	}

	// Test top-level Fetch func with nil options
	err = Fetch(ctx, nil)
	if err == nil {
		t.Fatal("expected error when calling Fetch with nil opts, got nil")
	}
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error containing %q, got: %v", expected, err)
	}
}

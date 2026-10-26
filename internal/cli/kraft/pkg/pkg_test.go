// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pkg

import (
	"context"
	"strings"
	"testing"

	"kraftkit.sh/config"
	"kraftkit.sh/packmanager"
	kraftfilev07 "unikraft.com/x/kraftfile"
)

func TestPkgNewCmdRegistration(t *testing.T) {
	cmd := NewCmd()
	if cmd == nil {
		t.Fatal("expected NewCmd to return non-nil command")
	}

	if cmd.Use != "pkg [FLAGS] [SUBCOMMAND|DIR]" {
		t.Errorf("unexpected cmd.Use: %q", cmd.Use)
	}

	expectedSubcommands := []string{
		"export",
		"info",
		"list",
		"pull",
		"push",
		"remove",
		"source",
		"unsource",
		"update",
	}

	subcmdMap := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subcmdMap[sub.Name()] = true
	}

	for _, expected := range expectedSubcommands {
		if !subcmdMap[expected] {
			t.Errorf("expected subcommand %q to be registered on pkg", expected)
		}
	}

	// Verify default flag values
	if f := cmd.Flag("as"); f == nil || f.DefValue != "oci" {
		t.Errorf("expected --as default to be 'oci', got %v", f)
	}

	if f := cmd.Flag("strategy"); f == nil || f.DefValue != "overwrite" {
		t.Errorf("expected --strategy default to be 'overwrite', got %v", f)
	}

	if f := cmd.Flag("rootfs-type"); f == nil || f.DefValue != "cpio" {
		t.Errorf("expected --rootfs-type default to be 'cpio', got %v", f)
	}

	if f := cmd.Flag("rom-type"); f == nil || f.DefValue != "cpio" {
		t.Errorf("expected --rom-type default to be 'cpio', got %v", f)
	}
}

func TestPkgValidation(t *testing.T) {
	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{
			NoPrompt: true,
		},
	})

	for _, tc := range []struct {
		name      string
		opts      PkgOptions
		expectErr string
	}{
		{
			name: "missing name",
			opts: PkgOptions{
				Workdir: t.TempDir(),
			},
			expectErr: "cannot package without setting --name",
		},
		{
			name: "target with platform conflict",
			opts: PkgOptions{
				Workdir:  t.TempDir(),
				Name:     "testpkg",
				Target:   "kvm-x86_64",
				Platform: "qemu",
			},
			expectErr: "the `--arch` and `--plat` options are not supported in addition to `--target`",
		},
		{
			name: "target with architecture conflict",
			opts: PkgOptions{
				Workdir:      t.TempDir(),
				Name:         "testpkg",
				Target:       "kvm-x86_64",
				Architecture: "x86_64",
			},
			expectErr: "the `--arch` and `--plat` options are not supported in addition to `--target`",
		},
		{
			name: "strategy prompt with no prompt setting",
			opts: PkgOptions{
				Workdir:  t.TempDir(),
				Name:     "testpkg",
				Strategy: packmanager.StrategyPrompt,
			},
			expectErr: "cannot mix --strategy=prompt when --no-prompt is enabled in settings",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Pkg(ctx, &tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.expectErr) {
				t.Fatalf("expected error containing %q, got: %v", tc.expectErr, err)
			}
		})
	}
}

func TestPkgPre(t *testing.T) {
	origUmbrella := packmanager.UmbrellaInstance
	packmanager.UmbrellaInstance = &packmanager.UmbrellaManager{}
	defer func() {
		packmanager.UmbrellaInstance = origUmbrella
	}()

	cmd := NewCmd()
	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})
	cmd.SetContext(ctx)

	// Test default Pre
	opts := PkgOptions{}
	if err := opts.Pre(cmd, nil); err != nil {
		t.Fatalf("unexpected error in opts.Pre: %v", err)
	}

	// Verify Pre updated cmd.Context with umbrella manager
	if cmd.Context() == ctx {
		t.Errorf("expected Pre to update cmd.Context with umbrella manager")
	}

	if opts.Strategy != packmanager.StrategyOverwrite {
		t.Errorf("expected default strategy %q, got %q", packmanager.StrategyOverwrite, opts.Strategy)
	}

	// When flags are unchanged, RootfsType and RomType must stay empty
	if opts.RootfsType != "" {
		t.Errorf("expected RootfsType to remain empty when unchanged, got %q", opts.RootfsType)
	}
	if opts.RomType != "" {
		t.Errorf("expected RomType to remain empty when unchanged, got %q", opts.RomType)
	}

	// Test custom rootfs-type flag
	if err := cmd.Flags().Set("rootfs-type", "erofs"); err != nil {
		t.Fatalf("failed to set rootfs-type flag: %v", err)
	}

	// Test custom rom-type flag
	if err := cmd.Flags().Set("rom-type", "erofs"); err != nil {
		t.Fatalf("failed to set rom-type flag: %v", err)
	}

	opts2 := PkgOptions{}
	if err := opts2.Pre(cmd, nil); err != nil {
		t.Fatalf("unexpected error in opts2.Pre: %v", err)
	}

	if opts2.RootfsType != kraftfilev07.FsTypeErofs {
		t.Errorf("expected RootfsType %q, got %q", kraftfilev07.FsTypeErofs, opts2.RootfsType)
	}
	if opts2.RomType != kraftfilev07.FsTypeErofs {
		t.Errorf("expected RomType %q, got %q", kraftfilev07.FsTypeErofs, opts2.RomType)
	}
}

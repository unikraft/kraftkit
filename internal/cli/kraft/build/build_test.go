// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"kraftkit.sh/cmdfactory"
	"kraftkit.sh/config"
	"kraftkit.sh/packmanager"
	kraftfilev07 "unikraft.com/x/kraftfile"
)

func initTestUmbrella(t *testing.T) {
	origUmbrella := packmanager.UmbrellaInstance
	packmanager.UmbrellaInstance = &packmanager.UmbrellaManager{}
	t.Cleanup(func() {
		packmanager.UmbrellaInstance = origUmbrella
	})
}

func TestBuildNewCmdRegistration(t *testing.T) {
	cmd := NewCmd()
	if cmd == nil {
		t.Fatal("expected NewCmd to return non-nil command")
	}

	if !strings.HasPrefix(cmd.Use, "build") {
		t.Errorf("unexpected cmd.Use: %q", cmd.Use)
	}

	// Verify alias 'bld'
	hasAlias := false
	for _, alias := range cmd.Aliases {
		if alias == "bld" {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		t.Errorf("expected cmd.Aliases to contain 'bld', got %v", cmd.Aliases)
	}

	// Verify AnnotationHelpGroup
	if cmd.Annotations[cmdfactory.AnnotationHelpGroup] != "build" {
		t.Errorf("expected AnnotationHelpGroup to be 'build', got %q", cmd.Annotations[cmdfactory.AnnotationHelpGroup])
	}

	// Verify primary flags registered
	for _, flag := range []string{
		"all",
		"arch",
		"config",
		"env",
		"toolchain",
		"toolchain-profile",
		"force-pull",
		"jobs",
		"kernel",
		"dbg",
		"kraftfile",
		"no-cache",
		"no-configure",
		"no-fast",
		"no-fetch",
		"no-rootfs",
		"no-update",
		"output",
		"plat",
		"print-stats",
		"rootfs",
		"rootfs-type",
		"keep-file-owners",
		"build-log",
		"target",
		"validate",
	} {
		if cmd.Flag(flag) == nil {
			t.Fatalf("expected --%s flag to be registered", flag)
		}
	}

	// Verify default rootfs-type is cpio
	if f := cmd.Flag("rootfs-type"); f == nil || f.DefValue != "cpio" {
		t.Errorf("expected default rootfs-type to be 'cpio', got %v", f)
	}
}

func TestBuildArgs(t *testing.T) {
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

func TestBuildPre(t *testing.T) {
	initTestUmbrella(t)

	cmd := NewCmd()
	opts := &BuildOptions{
		Rootfs: "relative/path/to/rootfs",
	}

	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})
	cmd.SetContext(ctx)

	if err := cmd.Flags().Set("rootfs-type", "erofs"); err != nil {
		t.Fatalf("failed to set rootfs-type flag: %v", err)
	}

	if err := opts.Pre(cmd, nil); err != nil {
		t.Fatalf("opts.Pre returned unexpected error: %v", err)
	}

	if opts.RootfsType != kraftfilev07.FsTypeErofs {
		t.Errorf("expected RootfsType to be %q, got %q", kraftfilev07.FsTypeErofs, opts.RootfsType)
	}

	if !filepath.IsAbs(opts.Rootfs) {
		t.Errorf("expected opts.Rootfs to be resolved to absolute path, got: %q", opts.Rootfs)
	}
}

func TestBuildPreInvalidToolchainProfile(t *testing.T) {
	initTestUmbrella(t)

	cmd := NewCmd()
	opts := &BuildOptions{
		ToolchainProfile: "non-existent-profile",
	}

	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})
	cmd.SetContext(ctx)

	err := opts.Pre(cmd, nil)
	if err == nil {
		t.Fatal("expected error for non-existent toolchain profile, got nil")
	}

	expected := "does not exist"
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("expected error containing %q, got: %v", expected, err)
	}
}

func TestBuildDefensiveNilOptions(t *testing.T) {
	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})

	tmpDir := t.TempDir()
	err := Build(ctx, nil, tmpDir)
	if err == nil {
		t.Fatal("expected error when running Build with empty args and nil opts, got nil")
	}

	if !errors.Is(err, ErrContextNotBuildable) {
		t.Errorf("expected ErrContextNotBuildable, got: %v", err)
	}
}

func TestBuildRunContextNotBuildable(t *testing.T) {
	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{},
	})

	tmpDir := t.TempDir()
	opts := &BuildOptions{
		Workdir: tmpDir,
	}

	err := Build(ctx, opts)
	if err == nil {
		t.Fatal("expected error when building empty directory, got nil")
	}

	if !errors.Is(err, ErrContextNotBuildable) {
		t.Errorf("expected ErrContextNotBuildable, got: %v", err)
	}
}

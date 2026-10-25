// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package remove

import (
	"context"
	"strings"
	"testing"

	"kraftkit.sh/config"
)

func TestRemoveValidation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		opts      RemoveOptions
		args      []string
		expectErr string
	}{
		{
			name:      "no machine specified without all",
			opts:      RemoveOptions{All: false},
			args:      []string{},
			expectErr: "no machine(s) specified",
		},
		{
			name:      "conflicting machine and all flag",
			opts:      RemoveOptions{All: true},
			args:      []string{"test-machine"},
			expectErr: "cannot specify machines and --all at the same time",
		},
		{
			name:      "nonexistent machine returns not found",
			opts:      RemoveOptions{All: false, Platform: "all"},
			args:      []string{"nonexistent-machine-xyz"},
			expectErr: "machine(s) not found",
		},
		{
			name:      "all flag with empty inventory succeeds",
			opts:      RemoveOptions{All: true, Platform: "all"},
			args:      []string{},
			expectErr: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
				Config: &config.KraftKit{RuntimeDir: t.TempDir()},
			})
			err := tc.opts.Run(ctx, tc.args)
			if tc.expectErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.expectErr) {
					t.Fatalf("expected error containing %q, got: %v", tc.expectErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
		})
	}
}

func TestRemoveAllEmptyInventory(t *testing.T) {
	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{RuntimeDir: t.TempDir()},
	})
	opts := RemoveOptions{
		All:      true,
		Platform: "all",
	}

	if err := opts.Run(ctx, nil); err != nil {
		t.Fatalf("expected nil error when removing all machines from empty inventory, got: %v", err)
	}
}

func TestRemoveCmdAllEmptyInventory(t *testing.T) {
	ctx := config.WithConfigManager(context.Background(), &config.ConfigManager[config.KraftKit]{
		Config: &config.KraftKit{RuntimeDir: t.TempDir()},
	})
	cmd := NewCmd()
	cmd.SetArgs([]string{"--all"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("expected nil error on kraft rm --all with empty inventory, got: %v", err)
	}
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"testing"
)

func TestManifestManager_IsCompatible_gitSources(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantCompat bool
	}{
		{
			name:       "github.com path",
			source:     "github.com/unikraft/lib-nginx.git",
			wantCompat: true,
		},
		{
			name:       "https github URL",
			source:     "https://github.com/unikraft/lib-nginx",
			wantCompat: true,
		},
		{
			name:       "git@ SSH URL",
			source:     "git@github.com:unikraft/lib-nginx.git",
			wantCompat: true,
		},
		{
			name:       "non-github .git suffix",
			source:     "https://gitlab.com/foo/bar.git",
			wantCompat: true,
		},
		{
			name:       "ssh:// URL without .git",
			source:     "ssh://git@github.com/owner/repo",
			wantCompat: true,
		},
		{
			name:       "git+ssh:// URL",
			source:     "git+ssh://github.com/owner/repo.git",
			wantCompat: true,
		},
		{
			name:       "empty source",
			source:     "",
			wantCompat: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ManifestManager{}
			ctx := context.Background()
			_, compat, err := m.IsCompatible(ctx, tt.source)
			if tt.wantCompat {
				if err != nil {
					t.Fatalf("IsCompatible(%q) unexpected error: %v", tt.source, err)
				}
				if !compat {
					t.Errorf("IsCompatible(%q) = false, want true", tt.source)
				}
			} else {
				if compat {
					t.Errorf("IsCompatible(%q) = true, want false", tt.source)
				}
			}
		})
	}
}

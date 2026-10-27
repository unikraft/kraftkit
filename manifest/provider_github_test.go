// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package manifest

import (
	"context"
	"testing"
)

func TestNewGitHubProviderURLParsing(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name         string
		path         string
		expectRepo   string
		expectOwner  string
		expectBranch string
		expectErr    bool
	}{
		{
			name:         "schemeless url with git extension",
			path:         "github.com/unikraft/lib-nginx.git",
			expectOwner:  "unikraft",
			expectRepo:   "lib-nginx",
			expectBranch: "",
			expectErr:    false,
		},
		{
			name:         "schemeless url with branch suffix",
			path:         "github.com/unikraft/lib-nginx.git@staging",
			expectOwner:  "unikraft",
			expectRepo:   "lib-nginx",
			expectBranch: "staging",
			expectErr:    false,
		},
		{
			name:         "https url with git extension",
			path:         "https://github.com/unikraft/app-sqlite.git",
			expectOwner:  "unikraft",
			expectRepo:   "app-sqlite",
			expectBranch: "",
			expectErr:    false,
		},
		{
			name:         "git ssh scp format",
			path:         "git@github.com:unikraft/kraftkit.git",
			expectOwner:  "unikraft",
			expectRepo:   "kraftkit",
			expectBranch: "",
			expectErr:    false,
		},
		{
			name:      "non-github host",
			path:      "https://gitlab.com/unikraft/lib-nginx.git",
			expectErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewGitHubProvider(ctx, tc.path)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for path %q, got nil", tc.path)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for path %q: %v", tc.path, err)
			}

			ghp, ok := p.(GitHubProvider)
			if !ok {
				t.Fatalf("expected GitHubProvider instance, got %T", p)
			}

			if ghp.repo.RepoOwner() != tc.expectOwner {
				t.Errorf("expected owner %q, got %q", tc.expectOwner, ghp.repo.RepoOwner())
			}

			if ghp.repo.RepoName() != tc.expectRepo {
				t.Errorf("expected repo %q, got %q", tc.expectRepo, ghp.repo.RepoName())
			}

			if ghp.branch != tc.expectBranch {
				t.Errorf("expected branch %q, got %q", tc.expectBranch, ghp.branch)
			}
		})
	}
}

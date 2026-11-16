// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ghrepo

import (
	"testing"
)

func TestNewFromURL(t *testing.T) {
	for _, tc := range []struct {
		name        string
		input       string
		expectOwner string
		expectRepo  string
		expectHost  string
		expectErr   bool
	}{
		{
			name:        "schemeless github url with git extension",
			input:       "github.com/unikraft/lib-nginx.git",
			expectOwner: "unikraft",
			expectRepo:  "lib-nginx",
			expectHost:  "github.com",
			expectErr:   false,
		},
		{
			name:        "schemeless github url without git extension",
			input:       "github.com/unikraft/lib-nginx",
			expectOwner: "unikraft",
			expectRepo:  "lib-nginx",
			expectHost:  "github.com",
			expectErr:   false,
		},
		{
			name:        "https github url with git extension",
			input:       "https://github.com/unikraft/app-sqlite.git",
			expectOwner: "unikraft",
			expectRepo:  "app-sqlite",
			expectHost:  "github.com",
			expectErr:   false,
		},
		{
			name:        "https github url without git extension",
			input:       "https://github.com/unikraft/app-sqlite",
			expectOwner: "unikraft",
			expectRepo:  "app-sqlite",
			expectHost:  "github.com",
			expectErr:   false,
		},
		{
			name:        "git ssh scp format",
			input:       "git@github.com:unikraft/kraftkit.git",
			expectOwner: "unikraft",
			expectRepo:  "kraftkit",
			expectHost:  "github.com",
			expectErr:   false,
		},
		{
			name:        "git protocol url",
			input:       "git://github.com/unikraft/unikraft.git",
			expectOwner: "unikraft",
			expectRepo:  "unikraft",
			expectHost:  "github.com",
			expectErr:   false,
		},
		{
			name:      "non-github host",
			input:     "https://gitlab.com/unikraft/lib-nginx.git",
			expectErr: true,
		},
		{
			name:      "invalid path structure",
			input:     "github.com/onlyowner",
			expectErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := NewFromURL(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tc.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}

			if r.RepoOwner() != tc.expectOwner {
				t.Errorf("expected owner %q, got %q", tc.expectOwner, r.RepoOwner())
			}

			if r.RepoName() != tc.expectRepo {
				t.Errorf("expected repo %q, got %q", tc.expectRepo, r.RepoName())
			}

			if r.RepoHost() != tc.expectHost {
				t.Errorf("expected host %q, got %q", tc.expectHost, r.RepoHost())
			}
		})
	}
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package ghrepo

import (
	"testing"
)

func TestNewFromURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "https without .git",
			input:     "https://github.com/unikraft/lib-nginx",
			wantOwner: "unikraft",
			wantRepo:  "lib-nginx",
		},
		{
			name:      "https with .git suffix",
			input:     "https://github.com/unikraft/lib-nginx.git",
			wantOwner: "unikraft",
			wantRepo:  "lib-nginx",
		},
		{
			name:      "ssh git@ URL",
			input:     "git@github.com:unikraft/lib-nginx.git",
			wantOwner: "unikraft",
			wantRepo:  "lib-nginx",
		},
		{
			name:      "ssh git@ URL without .git",
			input:     "git@github.com:unikraft/lib-nginx",
			wantOwner: "unikraft",
			wantRepo:  "lib-nginx",
		},
		{
			name:    "non-github host",
			input:   "https://gitlab.com/unikraft/lib-nginx",
			wantErr: true,
		},
		{
			name:      "bare github.com with .git",
			input:     "github.com/unikraft/lib-nginx.git",
			wantOwner: "unikraft",
			wantRepo:  "lib-nginx",
		},
		{
			name:      "bare github.com without .git",
			input:     "github.com/unikraft/lib-nginx",
			wantOwner: "unikraft",
			wantRepo:  "lib-nginx",
		},
		{
			name:    "missing repo segment",
			input:   "https://github.com/unikraft",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewFromURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewFromURL(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.RepoOwner() != tt.wantOwner {
				t.Errorf("RepoOwner() = %q, want %q", got.RepoOwner(), tt.wantOwner)
			}
			if got.RepoName() != tt.wantRepo {
				t.Errorf("RepoName() = %q, want %q", got.RepoName(), tt.wantRepo)
			}
		})
	}
}

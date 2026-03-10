// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import "testing"

func Test_isGitSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{
			name:   "github.com path prefix",
			source: "github.com/unikraft/lib-nginx.git",
			want:   true,
		},
		{
			name:   "https github URL",
			source: "https://github.com/unikraft/lib-nginx.git",
			want:   true,
		},
		{
			name:   "git@ SSH URL",
			source: "git@github.com:unikraft/lib-nginx.git",
			want:   true,
		},
		{
			name:   ".git suffix without github host",
			source: "gitlab.com/foo/bar.git",
			want:   true,
		},
		{
			name:   "plain OCI image ref",
			source: "unikraft.org/helloworld:latest",
			want:   false,
		},
		{
			name:   "docker.io image",
			source: "docker.io/library/nginx:latest",
			want:   false,
		},
		{
			name:   "registry hostname only",
			source: "index.unikraft.io",
			want:   false,
		},
		{
			name:   "image with digest",
			source: "unikraft.org/helloworld@sha256:abc123",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isGitSource(tt.source); got != tt.want {
				t.Errorf("isGitSource(%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}

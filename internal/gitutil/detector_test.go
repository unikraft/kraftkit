// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package gitutil

import "testing"

func TestIsGitSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   bool
	}{
		// SSH URL variants (should be git sources)
		{
			name:   "ssh:// URL without .git",
			source: "ssh://git@github.com/owner/repo",
			want:   true,
		},
		{
			name:   "ssh:// URL with .git",
			source: "ssh://git@github.com/owner/repo.git",
			want:   true,
		},
		{
			name:   "ssh+git:// URL",
			source: "ssh+git://github.com/owner/repo",
			want:   true,
		},
		{
			name:   "git+ssh:// URL",
			source: "git+ssh://github.com/owner/repo",
			want:   true,
		},
		{
			name:   "git:// URL",
			source: "git://github.com/owner/repo",
			want:   true,
		},
		{
			name:   "SCP-style git@host:path",
			source: "git@github.com:owner/repo",
			want:   true,
		},
		{
			name:   "SCP-style git@host:path with .git",
			source: "git@github.com:owner/repo.git",
			want:   true,
		},
		{
			name:   "SCP-style with GitLab",
			source: "git@gitlab.com:owner/repo.git",
			want:   true,
		},

		// HTTPS/HTTP git URLs (should be git sources)
		{
			name:   "HTTPS GitHub URL without .git",
			source: "https://github.com/owner/repo",
			want:   true,
		},
		{
			name:   "HTTPS GitHub URL with .git",
			source: "https://github.com/owner/repo.git",
			want:   true,
		},
		{
			name:   "HTTPS GitLab URL with .git",
			source: "https://gitlab.com/owner/repo.git",
			want:   true,
		},
		{
			name:   "HTTPS Bitbucket URL",
			source: "https://bitbucket.org/owner/repo.git",
			want:   true,
		},

		// Bare host/path patterns (should be git sources)
		{
			name:   "Bare github.com path without .git",
			source: "github.com/owner/repo",
			want:   true,
		},
		{
			name:   "Bare github.com path with .git",
			source: "github.com/owner/repo.git",
			want:   true,
		},
		{
			name:   "Bare gitlab.com path",
			source: "gitlab.com/owner/repo",
			want:   true,
		},
		{
			name:   "Bare bitbucket.org path",
			source: "bitbucket.org/owner/repo.git",
			want:   true,
		},

		// .git suffix on non-known hosts (should be git sources)
		{
			name:   ".git suffix on custom domain",
			source: "git.example.com/owner/repo.git",
			want:   true,
		},
		{
			name:   "HTTPS with .git on custom domain",
			source: "https://git.example.com/owner/repo.git",
			want:   true,
		},

		// OCI image references (should NOT be git sources)
		{
			name:   "OCI image with .git in name and tag",
			source: "ghcr.io/org/repo.git:latest",
			want:   false,
		},
		{
			name:   "OCI image with .git in name and digest",
			source: "ghcr.io/org/repo.git@sha256:abc123",
			want:   false,
		},
		{
			name:   "Docker Hub image",
			source: "docker.io/library/nginx:latest",
			want:   false,
		},
		{
			name:   "Plain OCI image with tag",
			source: "unikraft.org/helloworld:latest",
			want:   false,
		},
		{
			name:   "Plain OCI image with digest",
			source: "unikraft.org/helloworld@sha256:abc123def",
			want:   false,
		},
		{
			name:   "Registry hostname only",
			source: "index.unikraft.io",
			want:   false,
		},
		{
			name:   "Localhost registry with port",
			source: "localhost:5000/myimage:v1",
			want:   false,
		},
		{
			name:   "OCI ref with registry and tag",
			source: "registry.example.com/org/app:v1.0.0",
			want:   false,
		},

		// Edge cases
		{
			name:   "Empty string",
			source: "",
			want:   false,
		},
		{
			name:   "Just .git",
			source: ".git",
			want:   true,
		},
		{
			name:   "Path with .git in middle",
			source: "example.com/.git/repo",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsGitSource(tt.source); got != tt.want {
				t.Errorf("IsGitSource(%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}

func TestIsKnownGitHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		{
			name: "github.com",
			host: "github.com",
			want: true,
		},
		{
			name: "GitHub.com (case insensitive)",
			host: "GitHub.com",
			want: true,
		},
		{
			name: "gitlab.com",
			host: "gitlab.com",
			want: true,
		},
		{
			name: "bitbucket.org",
			host: "bitbucket.org",
			want: true,
		},
		{
			name: "github.com with port",
			host: "github.com:443",
			want: true,
		},
		{
			name: "subdomain of github.com",
			host: "api.github.com",
			want: true,
		},
		{
			name: "custom domain",
			host: "git.example.com",
			want: false,
		},
		{
			name: "docker.io",
			host: "docker.io",
			want: false,
		},
		{
			name: "empty host",
			host: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKnownGitHost(tt.host); got != tt.want {
				t.Errorf("isKnownGitHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

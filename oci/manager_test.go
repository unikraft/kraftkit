// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import "testing"

func Test_isGitSource_Wrapper(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{
			name:   "github.com path",
			source: "github.com/unikraft/lib-nginx.git",
			want:   true,
		},
		{
			name:   "ssh URL without .git",
			source: "ssh://git@github.com/owner/repo",
			want:   true,
		},
		{
			name:   "OCI image",
			source: "unikraft.org/helloworld:latest",
			want:   false,
		},
		{
			name:   "OCI with .git in name",
			source: "ghcr.io/org/repo.git:latest",
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

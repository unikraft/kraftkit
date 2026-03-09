// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package app

import (
	"testing"
)

func Test_normalizeProjectName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercase passthrough",
			input: "myapp",
			want:  "myapp",
		},
		{
			name:  "uppercase converted to lowercase",
			input: "MyApp",
			want:  "myapp",
		},
		{
			name:  "all uppercase",
			input: "MYAPP",
			want:  "myapp",
		},
		{
			name:  "with hyphen",
			input: "my-app",
			want:  "my-app",
		},
		{
			name:  "with underscore",
			input: "my_app",
			want:  "my_app",
		},
		{
			name:  "spaces removed",
			input: "my app",
			want:  "myapp",
		},
		{
			name:  "special characters removed",
			input: "my.app!",
			want:  "myapp",
		},
		{
			name:  "leading underscore trimmed",
			input: "_myapp",
			want:  "myapp",
		},
		{
			name:  "leading hyphen trimmed",
			input: "-myapp",
			want:  "myapp",
		},
		{
			name:  "multiple leading underscores and hyphens trimmed",
			input: "--__myapp",
			want:  "myapp",
		},
		{
			name:  "digits preserved",
			input: "app123",
			want:  "app123",
		},
		{
			name:  "mixed case with digits and special chars",
			input: "My App 123!",
			want:  "myapp123",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only special characters",
			input: "!!!",
			want:  "",
		},
		{
			name:  "only leading separators",
			input: "---",
			want:  "",
		},
		{
			name:  "uppercase with digits",
			input: "MyApp123",
			want:  "myapp123",
		},
		{
			name:  "trailing hyphen preserved",
			input: "myapp-",
			want:  "myapp-",
		},
		{
			name:  "trailing underscore preserved",
			input: "myapp_",
			want:  "myapp_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeProjectName(tt.input)
			if got != tt.want {
				t.Errorf("normalizeProjectName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

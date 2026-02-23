// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package set

import (
	"testing"
)

func TestContainsAnyOf(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		fields []string
		want   bool
	}{
		{
			name:   "nil values",
			args:   nil,
			fields: []string{"one", "two"},
		},
		{
			name:   "empty args",
			args:   []string{},
			fields: []string{"one", "two"},
		},
		{
			name:   "empty stringSet",
			args:   []string{"one", "two"},
			fields: []string{},
		},
		{
			name:   "partial match",
			args:   []string{"thr"},
			fields: []string{"one", "two", "three"},
			want:   true,
		},
		{
			name:   "single match",
			args:   []string{"zero", "two"},
			fields: []string{"one", "two", "three"},
			want:   true,
		},
		{
			name:   "case sensitivity",
			args:   []string{"ONE"},
			fields: []string{"one", "two", "three"},
		},
		{
			name:   "multiple match",
			args:   []string{"zero", "on", "two", "three"},
			fields: []string{"one", "two", "three"},
			want:   true,
		},
		{
			name:   "emoji",
			args:   []string{"🫩", "💀"},
			fields: []string{"😀", "💀🐹", "🐹", "😇", "😑"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStringSet(tt.fields...)
			if got := s.ContainsAnyOf(tt.args...); got != tt.want {
				t.Errorf("ContainsAnyOf() = %t, want %t", got, tt.want)
			}
		})
	}
}
func TestContainsExactlyAnyOf(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		fields []string
		want   bool
	}{
		{
			name:   "nil values",
			args:   nil,
			fields: []string{"one", "two"},
		},
		{
			name:   "empty args",
			args:   []string{},
			fields: []string{"one", "two"},
		},
		{
			name:   "empty stringSet",
			args:   []string{"one", "two"},
			fields: []string{},
		},
		{
			name:   "partial match",
			args:   []string{"thr"},
			fields: []string{"one", "two", "three"},
		},
		{
			name:   "single match",
			args:   []string{"zero", "two"},
			fields: []string{"one", "two", "three"},
			want:   true,
		},
		{
			name:   "case sensitivity",
			args:   []string{"ONE"},
			fields: []string{"one", "two", "three"},
		},
		{
			name:   "multiple match",
			args:   []string{"zero", "on", "two", "three"},
			fields: []string{"one", "two", "three"},
			want:   true,
		},
		{
			name:   "emoji",
			args:   []string{"🫩", "💀"},
			fields: []string{"😀", "💀", "🐹", "😇", "😑"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStringSet(tt.fields...)
			if got := s.ContainsExactlyAnyOf(tt.args...); got != tt.want {
				t.Errorf("ContainsExactlyAnyOf() = %t, want %t", got, tt.want)
			}
		})
	}
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package set

import (
	"reflect"
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

func TestContains(t *testing.T) {
	s := NewStringSet("one", "two", "three", "four")

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "full match",
			arg:  "one",
			want: true,
		},
		{
			name: "partial match",
			arg:  "tw",
			want: true,
		},
		{
			name: "incorrect match",
			arg:  "ten",
		},
		{
			name: "case sensitivity",
			arg:  "One",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.Contains(tt.arg); got != tt.want {
				t.Errorf("Contains() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestContainsExactly(t *testing.T) {
	s := NewStringSet("one", "two", "three", "four")

	tests := []struct {
		name string
		arg  string
		want bool
	}{
		{
			name: "full match",
			arg:  "one",
			want: true,
		},
		{
			name: "partial match",
			arg:  "tw",
		},
		{
			name: "incorrect match",
			arg:  "ten",
		},
		{
			name: "case sensitivity",
			arg:  "One",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.ContainsExactly(tt.arg); got != tt.want {
				t.Errorf("Contains() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestLen(t *testing.T) {
	s := NewStringSet()

	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}

	s.Add("one", "two")
	if s.Len() != 2 {
		t.Errorf("Len() = %d, want 2", s.Len())
	}
}

func TestToSlice(t *testing.T) {
	s := NewStringSet()

	got := s.ToSlice()
	want := []string{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToSlice() = %v, want %v", got, want)
	}

	s.Add("one", "two")
	got = s.ToSlice()
	want = []string{"one", "two"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToSlice() = %v, want %v", got, want)
	}
}

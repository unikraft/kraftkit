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
			name:   "nil args",
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
			name:   "nil args",
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

func TestEqual(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		fields []string
		want   bool
	}{
		{
			name:   "nil args",
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
			name:   "equal in order",
			args:   []string{"one", "two", "three"},
			fields: []string{"one", "two", "three"},
			want:   true,
		},
		{
			name:   "equal out of order",
			args:   []string{"three", "one", "two"},
			fields: []string{"one", "two", "three"},
			want:   true,
		},
		{
			name:   "case sensitivity",
			args:   []string{"ONE", "tWo", "three"},
			fields: []string{"one", "two", "three"},
		},
		{
			name:   "partial set match",
			args:   []string{"one", "three"},
			fields: []string{"one", "two", "three"},
		},
		{
			name:   "partial string match",
			args:   []string{"on", "wo", "hree"},
			fields: []string{"one", "two", "three"},
		},
		{
			name:   "emoji",
			args:   []string{"😀", "💀🐹", "🐹"},
			fields: []string{"😀", "💀🐹", "🐹"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s1 := NewStringSet(tt.fields...)
			s2 := NewStringSet(tt.args...)
			if got := s1.Equal(s2); got != tt.want {
				t.Errorf("Equal() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestSliceWithout(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		without string
		want    []string
	}{
		{
			name:    "nil args",
			args:    nil,
			without: "",
			want:    nil,
		},
		{
			name:    "empty args",
			args:    []string{},
			without: "",
			want:    []string{},
		},
		{
			name:    "remove first",
			args:    []string{"one", "two", "three"},
			without: "one",
			want:    []string{"two", "three"},
		},
		{
			name:    "remove from multiple instances",
			args:    []string{"one", "two", "three", "two"},
			without: "two",
			want:    []string{"one", "three", "two"},
		},
		{
			name:    "remove arbitrary from even",
			args:    []string{"one", "two", "three", "four"},
			without: "three",
			want:    []string{"one", "two", "four"},
		},
		{
			name:    "remove arbitrary from odd",
			args:    []string{"one", "two", "three", "four", "five"},
			without: "three",
			want:    []string{"one", "two", "four", "five"},
		},
		{
			name:    "case sensitivity",
			args:    []string{"one", "two", "three"},
			without: "ONE",
			want:    []string{"one", "two", "three"},
		},
		{
			name:    "remove last",
			args:    []string{"one", "two", "three"},
			without: "three",
			want:    []string{"one", "two"},
		},
		{
			name:    "emoji",
			args:    []string{"😀", "💀🐹", "🐹"},
			without: "💀🐹",
			want:    []string{"😀", "🐹"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sliceWithout(tt.args, tt.without)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sliceWithout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name      string
		initial   []string
		args      []string
		wantSlice []string
	}{
		{
			name:      "nil args",
			args:      nil,
			initial:   []string{"one", "two"},
			wantSlice: []string{"one", "two"},
		},
		{
			name:      "empty args",
			args:      []string{},
			initial:   []string{"one", "two"},
			wantSlice: []string{"one", "two"},
		},
		{
			name:      "empty stringSet",
			args:      []string{"one", "two"},
			initial:   []string{},
			wantSlice: []string{"one", "two"},
		},
		{
			name:      "single args",
			args:      []string{"three"},
			initial:   []string{"one", "two"},
			wantSlice: []string{"one", "two", "three"},
		},
		{
			name:      "multiple args",
			args:      []string{"two", "three"},
			initial:   []string{"one"},
			wantSlice: []string{"one", "two", "three"},
		},
		{
			name:      "already exists",
			args:      []string{"two"},
			initial:   []string{"one", "two"},
			wantSlice: []string{"one", "two"},
		},
		{
			name:      "multiple repeating args",
			args:      []string{"two", "two", "three"},
			initial:   []string{"one"},
			wantSlice: []string{"one", "two", "three"},
		},
		{
			name:      "emoji",
			args:      []string{"🫩"},
			initial:   []string{"😀", "💀🐹", "🐹", "😇", "😑"},
			wantSlice: []string{"😀", "💀🐹", "🐹", "😇", "😑", "🫩"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewStringSet(tt.initial...).Add(tt.args...)
			want := NewStringSet(tt.wantSlice...)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Add() = %v, want %v", got, tt.wantSlice)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name      string
		initial   []string
		args      []string
		wantSlice []string
	}{
		{
			name:      "nil args",
			args:      nil,
			initial:   []string{"one", "two"},
			wantSlice: []string{"one", "two"},
		},
		{
			name:      "empty args",
			args:      []string{},
			initial:   []string{"one", "two"},
			wantSlice: []string{"one", "two"},
		},
		{
			name:      "empty stringSet",
			args:      []string{"one", "two"},
			initial:   []string{},
			wantSlice: []string{},
		},
		{
			name:      "single args",
			args:      []string{"three"},
			initial:   []string{"one", "two", "three"},
			wantSlice: []string{"one", "two"},
		},
		{
			name:      "multiple args",
			args:      []string{"one", "three"},
			initial:   []string{"one", "two", "three"},
			wantSlice: []string{"two"},
		},
		{
			name:      "doesn't exist",
			args:      []string{"three"},
			initial:   []string{"one", "two"},
			wantSlice: []string{"one", "two"},
		},
		{
			name:      "repeating args",
			args:      []string{"two", "two"},
			initial:   []string{"one", "two"},
			wantSlice: []string{"one"},
		},
		{
			name:      "partial match",
			args:      []string{"tw", "thr"},
			initial:   []string{"one", "two", "three"},
			wantSlice: []string{"one", "two", "three"},
		},
		{
			name:      "emoji",
			args:      []string{"😇"},
			initial:   []string{"😀", "💀🐹", "🐹", "😇", "😑"},
			wantSlice: []string{"😀", "💀🐹", "🐹", "😑"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewStringSet(tt.initial...).Remove(tt.args...)
			want := NewStringSet(tt.wantSlice...)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Add() = %v, want %v", got, tt.wantSlice)
			}
		})
	}
}

func TestNewStringSet(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		wantSlice []string
	}{
		{
			name:      "nil args",
			input:     nil,
			wantSlice: []string{},
		},
		{
			name:      "empty args",
			input:     []string{},
			wantSlice: []string{},
		},
		{
			name:      "single input",
			input:     []string{"one"},
			wantSlice: []string{"one"},
		},
		{
			name:      "multiple input",
			input:     []string{"one", "two"},
			wantSlice: []string{"one", "two"},
		},
		{
			name:      "repeating input values",
			input:     []string{"one", "two", "three", "two"},
			wantSlice: []string{"one", "two", "three"},
		},
		{
			name:      "emoji",
			input:     []string{"😀", "💀🐹", "🐹", "😑"},
			wantSlice: []string{"😀", "💀🐹", "🐹", "😑"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewStringSet(tt.input...)

			if !reflect.DeepEqual(got.v, tt.wantSlice) {
				t.Errorf("NewStringSet() = %v, want %v", got.v, tt.wantSlice)
			}

			if len(got.m) != len(tt.wantSlice) {
				t.Errorf("map size %d, expected %d", len(got.m), len(tt.wantSlice))
			}

			for _, s := range tt.wantSlice {
				if _, ok := got.m[s]; !ok {
					t.Errorf("expected value %q in map: %v", s, got.m)
				}
			}
		})
	}
}

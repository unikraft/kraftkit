// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package kconfig

import (
	"reflect"
	"testing"
)

func TestReadNextLine(t *testing.T) {
	tests := []struct {
		name       string
		inputData  []byte
		outputData []byte
		want       string
	}{
		{
			name:       "empty data",
			inputData:  []byte(""),
			outputData: nil,
			want:       "",
		},
		{
			name:       "single line",
			inputData:  []byte("one two three\n"),
			outputData: []byte(""),
			want:       "one two three",
		},
		{
			name:       "multi line",
			inputData:  []byte("one two \nthree\n"),
			outputData: []byte("three\n"),
			want:       "one two ",
		},
		{
			name:       "non ascii unicode safety",
			inputData:  []byte("🫩💀🐹\n😇"),
			outputData: []byte("😇"),
			want:       "🫩💀🐹",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newParser(tt.inputData, "", "", nil)
			got := p.readNextLine()

			if got != tt.want {
				t.Errorf("readNextLine() = %q, want %q", got, tt.want)
			}
			if !reflect.DeepEqual(p.data, tt.outputData) {
				t.Errorf("data = %q, expected %q", p.data, tt.outputData)
			}
		})
	}
}

func TestSkipSpaces(t *testing.T) {
	tests := []struct {
		name    string
		col     int
		current string
		want    int
	}{
		{
			name:    "empty line",
			current: "",
			col:     0,
			want:    0,
		},
		{
			name:    "no spaces",
			current: "one",
			col:     0,
			want:    0,
		},
		{
			name:    "only spaces",
			current: "    ",
			col:     0,
			want:    4,
		},
		{
			name:    "start of the line",
			current: "   \tone two three",
			col:     0,
			want:    4,
		},
		{
			name:    "middle of the line",
			current: "one two three",
			col:     3,
			want:    4,
		},
		{
			name:    "already out of bounds",
			current: "one",
			col:     3,
			want:    3,
		},
		{
			name:    "non ascii unicode",
			current: "🫩💀🐹 😇",
			col:     12, // emojis are 4 bytes long in UTF-8
			want:    13,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &parser{
				current: tt.current,
				col:     tt.col,
			}

			p.skipSpaces()

			if p.col != tt.want {
				t.Errorf("skipSpaces() = %v, want %v", p.col, tt.want)
			}
		})
	}
}

func TestIndentLevel(t *testing.T) {
	tests := []struct {
		name    string
		col     int
		current string
		want    int
	}{
		{
			name:    "empty line",
			current: "",
			col:     0,
			want:    0,
		},
		{
			name:    "no indentation",
			current: "one",
			col:     1, // cursor reads 'n'
			want:    1,
		},
		{
			name:    "leading spaces",
			current: "  one",
			col:     2,
			want:    2,
		},
		{
			name:    "mixed space and tab",
			current: "  \tone",
			col:     3,
			want:    8, // rounded to nearest multiple of 8
		},
		{
			name:    "only \\t indent",
			current: "\t\t\tone",
			col:     3,
			want:    24,
		},
		{
			name:    "space in between of line",
			current: "\tone two",
			col:     5,  // cursor at 't'
			want:    12, // 8 + 3 + 1
		},
		{
			name:    "indent in between of line",
			current: "\tone\ttwo",
			col:     5,
			want:    16,
		},
		{
			name:    "non ascii unicode",
			current: "\t🫩💀🐹",
			col:     5,  // points to '💀'
			want:    12, // 8 + 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &parser{
				current: tt.current,
				col:     tt.col,
			}

			got := p.indentLevel()

			if got != tt.want {
				t.Errorf("indentLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

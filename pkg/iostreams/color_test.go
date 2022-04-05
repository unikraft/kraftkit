// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package iostreams

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvColorDisabled(t *testing.T) {
	orig_NO_COLOR := os.Getenv("NO_COLOR")
	orig_CLICOLOR := os.Getenv("CLICOLOR")
	orig_CLICOLOR_FORCE := os.Getenv("CLICOLOR_FORCE")
	t.Cleanup(func() {
		os.Setenv("NO_COLOR", orig_NO_COLOR)
		os.Setenv("CLICOLOR", orig_CLICOLOR)
		os.Setenv("CLICOLOR_FORCE", orig_CLICOLOR_FORCE)
	})

	tests := []struct {
		name           string
		NO_COLOR       string
		CLICOLOR       string
		CLICOLOR_FORCE string
		want           bool
	}{
		{
			name:           "pristine env",
			NO_COLOR:       "",
			CLICOLOR:       "",
			CLICOLOR_FORCE: "",
			want:           false,
		},
		{
			name:           "NO_COLOR enabled",
			NO_COLOR:       "1",
			CLICOLOR:       "",
			CLICOLOR_FORCE: "",
			want:           true,
		},
		{
			name:           "CLICOLOR disabled",
			NO_COLOR:       "",
			CLICOLOR:       "0",
			CLICOLOR_FORCE: "",
			want:           true,
		},
		{
			name:           "CLICOLOR enabled",
			NO_COLOR:       "",
			CLICOLOR:       "1",
			CLICOLOR_FORCE: "",
			want:           false,
		},
		{
			name:           "CLICOLOR_FORCE has no effect",
			NO_COLOR:       "",
			CLICOLOR:       "",
			CLICOLOR_FORCE: "1",
			want:           false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("NO_COLOR", tt.NO_COLOR)
			os.Setenv("CLICOLOR", tt.CLICOLOR)
			os.Setenv("CLICOLOR_FORCE", tt.CLICOLOR_FORCE)

			if got := EnvColorDisabled(); got != tt.want {
				t.Errorf("EnvColorDisabled(): want %v, got %v", tt.want, got)
			}
		})
	}
}

func TestEnvColorForced(t *testing.T) {
	orig_NO_COLOR := os.Getenv("NO_COLOR")
	orig_CLICOLOR := os.Getenv("CLICOLOR")
	orig_CLICOLOR_FORCE := os.Getenv("CLICOLOR_FORCE")
	t.Cleanup(func() {
		os.Setenv("NO_COLOR", orig_NO_COLOR)
		os.Setenv("CLICOLOR", orig_CLICOLOR)
		os.Setenv("CLICOLOR_FORCE", orig_CLICOLOR_FORCE)
	})

	tests := []struct {
		name           string
		NO_COLOR       string
		CLICOLOR       string
		CLICOLOR_FORCE string
		want           bool
	}{
		{
			name:           "pristine env",
			NO_COLOR:       "",
			CLICOLOR:       "",
			CLICOLOR_FORCE: "",
			want:           false,
		},
		{
			name:           "NO_COLOR enabled",
			NO_COLOR:       "1",
			CLICOLOR:       "",
			CLICOLOR_FORCE: "",
			want:           false,
		},
		{
			name:           "CLICOLOR disabled",
			NO_COLOR:       "",
			CLICOLOR:       "0",
			CLICOLOR_FORCE: "",
			want:           false,
		},
		{
			name:           "CLICOLOR enabled",
			NO_COLOR:       "",
			CLICOLOR:       "1",
			CLICOLOR_FORCE: "",
			want:           false,
		},
		{
			name:           "CLICOLOR_FORCE enabled",
			NO_COLOR:       "",
			CLICOLOR:       "",
			CLICOLOR_FORCE: "1",
			want:           true,
		},
		{
			name:           "CLICOLOR_FORCE disabled",
			NO_COLOR:       "",
			CLICOLOR:       "",
			CLICOLOR_FORCE: "0",
			want:           false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("NO_COLOR", tt.NO_COLOR)
			os.Setenv("CLICOLOR", tt.CLICOLOR)
			os.Setenv("CLICOLOR_FORCE", tt.CLICOLOR_FORCE)

			if got := EnvColorForced(); got != tt.want {
				t.Errorf("EnvColorForced(): want %v, got %v", tt.want, got)
			}
		})
	}
}

func Test_HextoRGB(t *testing.T) {
	tests := []struct {
		name  string
		hex   string
		text  string
		wants string
		cs    *ColorScheme
	}{
		{
			name:  "truecolor",
			hex:   "fc0303",
			text:  "red",
			wants: "\033[38;2;252;3;3mred\033[0m",
			cs:    NewColorScheme(true, true, true),
		},
		{
			name:  "no truecolor",
			hex:   "fc0303",
			text:  "red",
			wants: "red",
			cs:    NewColorScheme(true, true, false),
		},
		{
			name:  "no color",
			hex:   "fc0303",
			text:  "red",
			wants: "red",
			cs:    NewColorScheme(false, false, false),
		},
	}

	for _, tt := range tests {
		output := tt.cs.HexToRGB(tt.hex, tt.text)
		assert.Equal(t, tt.wants, output)
	}
}

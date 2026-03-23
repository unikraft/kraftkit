// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2022 Unikraft UG.
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

package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFuzzyAgo(t *testing.T) {
	cases := map[string]string{
		"1s":         "less than a minute ago",
		"30s":        "less than a minute ago",
		"1m08s":      "about 1 minute ago",
		"15m0s":      "about 15 minutes ago",
		"59m10s":     "about 59 minutes ago",
		"1h10m02s":   "about 1 hour ago",
		"15h0m01s":   "about 15 hours ago",
		"30h10m":     "about 1 day ago",
		"50h":        "about 2 days ago",
		"720h05m":    "about 1 month ago",
		"3000h10m":   "about 4 months ago",
		"8760h59m":   "about 1 year ago",
		"17601h59m":  "about 2 years ago",
		"262800h19m": "about 30 years ago",
	}

	for duration, expected := range cases {
		d, e := time.ParseDuration(duration)
		if e != nil {
			t.Errorf("failed to create a duration: %s", e)
		}

		fuzzy := FuzzyAgo(d)
		if fuzzy != expected {
			t.Errorf("unexpected fuzzy duration value: %s for %s", fuzzy, duration)
		}
	}
}

func TestFuzzyAgoAbbr(t *testing.T) {
	const form = "2006-Jan-02 15:04:05"
	now, _ := time.Parse(form, "2020-Nov-22 14:00:00")

	cases := map[string]string{
		"2020-Nov-22 14:00:00": "0m",
		"2020-Nov-22 13:59:00": "1m",
		"2020-Nov-22 13:30:00": "30m",
		"2020-Nov-22 13:00:00": "1h",
		"2020-Nov-22 02:00:00": "12h",
		"2020-Nov-21 14:00:00": "1d",
		"2020-Nov-07 14:00:00": "15d",
		"2020-Oct-24 14:00:00": "29d",
		"2020-Oct-23 14:00:00": "Oct 23, 2020",
		"2019-Nov-22 14:00:00": "Nov 22, 2019",
	}

	for createdAt, expected := range cases {
		d, _ := time.Parse(form, createdAt)
		fuzzy := FuzzyAgoAbbr(now, d)
		if fuzzy != expected {
			t.Errorf("unexpected fuzzy duration abbr value: %s for %s", fuzzy, createdAt)
		}
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		num      int
		thing    string
		expected string
	}{
		{1, "item", "1 item"},
		{0, "item", "0 items"},
		{2, "item", "2 items"},
		{100, "file", "100 files"},
	}
	for _, tt := range tests {
		got := Pluralize(tt.num, tt.thing)
		if got != tt.expected {
			t.Errorf("Pluralize(%d, %q) = %q, want %q", tt.num, tt.thing, got, tt.expected)
		}
	}
}

func TestHumanize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello_world", "hello world"},
		{"hello-world", "hello world"},
		{"hello_world-foo", "hello world foo"},
		{"nospace", "nospace"},
		{"", ""},
	}
	for _, tt := range tests {
		got := Humanize(tt.input)
		if got != tt.expected {
			t.Errorf("Humanize(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"http://example.com", true},
		{"https://example.com", true},
		{"ftp://example.com", false},
		{"example.com", false},
		{"", false},
		{"/local/path", false},
	}
	for _, tt := range tests {
		got := IsURL(tt.input)
		if got != tt.expected {
			t.Errorf("IsURL(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestDisplayURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://example.com/path", "example.com/path"},
		{"http://foo.bar/baz/qux", "foo.bar/baz/qux"},
		{"not a url %%", "not a url %%"},
	}
	for _, tt := range tests {
		got := DisplayURL(tt.input)
		if got != tt.expected {
			t.Errorf("DisplayURL(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestValidURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"https://example.com", true},
		{"", true},
		{strings.Repeat("a", 8191), true},
		{strings.Repeat("a", 8192), false},
		{strings.Repeat("a", 9000), false},
	}
	for _, tt := range tests {
		got := ValidURL(tt.input)
		if got != tt.expected {
			t.Errorf("ValidURL(len=%d) = %v, want %v", len(tt.input), got, tt.expected)
		}
	}
}

func TestListJoinStr(t *testing.T) {
	tests := []struct {
		items    []string
		delim    string
		expected string
	}{
		{[]string{"a", "b", "c"}, ", ", "a, b, c"},
		{[]string{"x"}, ", ", "x"},
		{[]string{}, ", ", ""},
		{[]string{"a", "b"}, "-", "a-b"},
	}
	for _, tt := range tests {
		got := ListJoinStr(tt.items, tt.delim)
		if got != tt.expected {
			t.Errorf("ListJoinStr(%v, %q) = %q, want %q", tt.items, tt.delim, got, tt.expected)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		haystack []string
		needle   string
		expected bool
	}{
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
		{[]string{"foo"}, "foo", true},
	}
	for _, tt := range tests {
		got := Contains(tt.haystack, tt.needle)
		if got != tt.expected {
			t.Errorf("Contains(%v, %q) = %v, want %v", tt.haystack, tt.needle, got, tt.expected)
		}
	}
}

func TestHumanizeDuration(t *testing.T) {
	tests := []struct {
		dur      time.Duration
		expected string
	}{
		{0, "0.0s"},
		{500 * time.Millisecond, "0.5s"},
		{1*time.Second + 500*time.Millisecond, "1.5s"},
		{65 * time.Second, "1m 5s"},
		{75 * time.Second, "1m 15s"},
		{10 * time.Minute, "10m  0s"},
		{10*time.Minute + 5*time.Second, "10m  5s"},
		{1*time.Hour + 1*time.Minute + 1*time.Second, "1h  1m  1s"},
		{2*time.Hour + 30*time.Minute + 45*time.Second, "2h 30m 45s"},
	}
	for _, tt := range tests {
		got := HumanizeDuration(tt.dur)
		if got != tt.expected {
			t.Errorf("HumanizeDuration(%v) = %q, want %q", tt.dur, got, tt.expected)
		}
	}
}

func TestRelativePath_absolute(t *testing.T) {
	got := RelativePath("/some/base", "/absolute/path")
	if got != "/absolute/path" {
		t.Errorf("expected /absolute/path, got %s", got)
	}
}

func TestRelativePath_relative(t *testing.T) {
	got := RelativePath("/base/dir", "subdir/file.txt")
	expected := filepath.Join("/base/dir", "subdir/file.txt")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

func TestRelativePath_tilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := RelativePath("/any/base", "~/myfile.txt")
	expected := filepath.Join(home, "myfile.txt")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}
}

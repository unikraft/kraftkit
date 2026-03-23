// SPDX-License-Identifier: MIT
//
// Copyright (c) 2019 GitHub Inc.
//               2025 Unikraft GmbH.
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
	"testing"
)

func TestIsCygwinTerminal_regularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "terminal_test_*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()

	// A regular file is never a Cygwin terminal
	result := IsCygwinTerminal(f)
	if result {
		t.Errorf("expected IsCygwinTerminal to return false for a regular file")
	}
}

func TestIsTerminal_regularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "terminal_test_*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()

	// A regular file is never a terminal
	result := IsTerminal(f)
	if result {
		t.Errorf("expected IsTerminal to return false for a regular file")
	}
}

func TestTerminalSize_nonFile(t *testing.T) {
	// Passing a non-*os.File value should return an error
	w, h, err := TerminalSize("not a file")
	if err == nil {
		t.Errorf("expected error for non-file input, got w=%d h=%d", w, h)
	}
}

func TestTerminalSize_regularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "terminal_test_*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()

	// A regular file is not a TTY, so GetSize will fail or return 0,0
	_, _, err = TerminalSize(f)
	// We just verify it doesn't panic; error is expected on non-tty fd
	_ = err
}

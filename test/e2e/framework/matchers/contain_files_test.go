// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package matchers_test

import (
	"os"
	"path/filepath"
	"testing"

	"kraftkit.sh/test/e2e/framework/matchers"
)

func TestContainFilesMatcher(t *testing.T) {
	const f1 = "f1.txt"
	const f2 = "f2.txt"

	testCases := []struct {
		desc    string
		files   []fileEntry
		success bool
	}{
		{
			desc:    "All files exist",
			files:   []fileEntry{regular(f1), regular(f2), regular("other.txt")},
			success: true,
		}, {
			desc:    "One file is missing",
			files:   []fileEntry{regular(f1), regular("other1.txt"), regular("other2.txt")},
			success: false,
		}, {
			desc:    "All files are missing",
			files:   []fileEntry{regular("other1.txt"), regular("other2.txt")},
			success: false,
		}, {
			desc:    "One file is a directory",
			files:   []fileEntry{regular(f1), dir(f2)},
			success: false,
		}, {
			desc:    "One file is a symbolic link",
			files:   []fileEntry{regular(f1), symlink(f2)},
			success: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			d := newDirectory(t, tc.files...)

			m := matchers.ContainFiles(f1, f2)
			success, err := m.Match(d)
			if err != nil {
				t.Fatal("Failed to run matcher:", err)
			}

			if tc.success && !success {
				t.Error("Expected the matcher to succeed. Failure was:", m.FailureMessage(d))
			} else if !tc.success && success {
				t.Error("Expected the matcher to fail")
			}
		})
	}
}

// newDirectory creates a new temporary directory containing the given files.
// A cleanup function is automatically registered to remove the directory and
// all its children from the filesystem at the end of the given test.
func newDirectory(t *testing.T, files ...fileEntry) (path string) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "kraftkit-e2e-matchers-*")
	if err != nil {
		t.Fatal("Failed to create temporary directory:", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Fatal("Failed to remove temporary directory:", err)
		}
	})

	for _, fe := range files {
		fp := filepath.Join(tmpDir, fe.name)

		switch ft := fe.typ; ft {
		case fileTypeRegular:
			f, err := os.OpenFile(fp, os.O_CREATE|os.O_TRUNC, 0o600)
			if err != nil {
				t.Fatalf("Failed to create regular file %q: %s", fe.name, err)
			}
			f.Close()

		case fileTypeDirectory:
			if err := os.Mkdir(fp, 0o700); err != nil {
				t.Fatalf("Failed to create directory %q: %s", fe.name, err)
			}

		case fileTypeSymlink:
			if err := os.Symlink("/dev/null", fp); err != nil {
				t.Fatalf("Failed to create symbolic link %q: %s", fe.name, err)
			}

		default:
			t.Fatalf("Unknown type %v for file %q", ft, fe.name)
		}
	}

	return tmpDir
}

// fileEntry represents a file to create in a directory used for tests.
type fileEntry struct {
	name string
	typ  fileType
}

// regular returns a fileEntry for a regular file.
func regular(name string) fileEntry {
	return fileEntry{name: name, typ: fileTypeRegular}
}

// dir returns a fileEntry for a directory.
func dir(name string) fileEntry {
	return fileEntry{name: name, typ: fileTypeDirectory}
}

// symlink returns a fileEntry for a symbolic link.
func symlink(name string) fileEntry {
	return fileEntry{name: name, typ: fileTypeSymlink}
}

type fileType uint8

const (
	fileTypeUnknown fileType = iota
	fileTypeRegular
	fileTypeDirectory
	fileTypeSymlink
)

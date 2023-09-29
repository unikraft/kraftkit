// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd_test

import (
	"io"
	"os"
	"testing"

	"github.com/cavaliergopher/cpio"

	"kraftkit.sh/initrd"
)

func TestNewFromMapping(t *testing.T) {
	const rootDir = "testdata/rootfs"

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal("Failed to get current working directory:", err)
	}

	tmpFile := newTempFile(t)
	initdCfg, err := initrd.NewFromMapping(cwd, tmpFile, rootDir+":/")
	if err != nil {
		t.Fatal("NewFromMapping:", err)
	}

	const expectFiles = 4 // only regular and symlink files are indexed
	if gotFiles := len(initdCfg.Files); gotFiles != expectFiles {
		t.Errorf("Expected %d files in InitrdConfig, got %d: %v", expectFiles, gotFiles, initdCfg.Files)
	}

	r, err := initdCfg.NewReader(openFile(t, tmpFile))
	if err != nil {
		t.Fatal("NewReader:", err)
	}

	expectHeaders := map[string]cpio.Header{
		".": {
			Mode: cpio.ModeDir,
		},
		"entrypoint.sh": {
			Mode: cpio.ModeRegular,
			Size: 25,
		},
		"etc": {
			Mode: cpio.ModeDir,
		},
		"etc/app.conf": {
			Mode: cpio.ModeRegular,
			Size: 16,
		},
		"lib": {
			Mode: cpio.ModeDir,
		},
		"lib/libtest.so.1": {
			Mode:     cpio.ModeSymlink,
			Linkname: "libtest.so.1.0.0",
		},
		"lib/libtest.so.1.0.0": {
			Mode: cpio.ModeRegular,
			Size: 9,
		},
	}

	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("Failed to read next cpio header:", err)
		}

		expectHdr, ok := expectHeaders[hdr.Name]
		if !ok {
			t.Error("Encountered unexpected file in cpio archive:", hdr.Name)
			continue
		}

		if gotMode := hdr.Mode & cpio.ModeType; gotMode != expectHdr.Mode {
			t.Errorf("file [%s]: got mode %s, expected %s", hdr.Name, gotMode, expectHdr.Mode)
		}
		if hdr.Linkname != expectHdr.Linkname {
			t.Errorf("file [%s]: got linkname %q, expected %q", hdr.Name, hdr.Linkname, expectHdr.Linkname)
		}
		if hdr.Size != expectHdr.Size {
			t.Errorf("file [%s]: got size %d, expected %d", hdr.Name, hdr.Size, expectHdr.Size)
		}
	}
}

// newTempFile creates a temporary file, and removes it when the test completes.
func newTempFile(t *testing.T) string {
	t.Helper()

	f, err := os.CreateTemp("", "kraftkit-test-initrd-")
	if err != nil {
		t.Fatal("Failed to create temporary file:", err)
	}
	_ = f.Close()

	filePath := f.Name()

	t.Cleanup(func() {
		if err := os.Remove(filePath); err != nil {
			t.Fatalf("Failed to remove temporary file %s: %v", filePath, err)
		}
	})

	return filePath
}

// openFile opens a file for reading, and closes it when the test completes.
func openFile(t *testing.T, path string) io.Reader {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal("Failed to open file for reading:", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	return f
}

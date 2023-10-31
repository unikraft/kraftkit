// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd_test

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/cavaliergopher/cpio"

	"kraftkit.sh/initrd"
)

func TestNewFromDirectory(t *testing.T) {
	const rootDir = "testdata/rootfs"

	ctx := context.Background()

	ird, err := initrd.NewFromDirectory(ctx, rootDir)
	if err != nil {
		t.Fatal("NewFromDirectory:", err)
	}

	irdPath, err := ird.Build(ctx)
	if err != nil {
		t.Fatal("Build:", err)
	}
	t.Cleanup(func() {
		if err := os.Remove(irdPath); err != nil {
			t.Fatal("Failed to remove initrd file:", err)
		}
	})

	irdFiles := ird.Files()

	const expectFiles = 4 // only regular and symlink files are indexed
	if gotFiles := len(irdFiles); gotFiles != expectFiles {
		t.Errorf("Expected %d files in InitrdConfig, got %d: %v", expectFiles, gotFiles, irdFiles)
	}

	r := cpio.NewReader(openFile(t, irdPath))

	expectHeaders := map[string]cpio.Header{
		"/entrypoint.sh": {
			Mode: cpio.TypeReg,
			Size: 25,
		},
		"/etc": {
			Mode: cpio.TypeDir,
		},
		"/etc/app.conf": {
			Mode: cpio.TypeReg,
			Size: 16,
		},
		"/lib": {
			Mode: cpio.TypeDir,
		},
		"/lib/libtest.so.1": {
			Mode:     cpio.TypeSymlink,
			Linkname: "libtest.so.1.0.0",
		},
		"/lib/libtest.so.1.0.0": {
			Mode: cpio.TypeReg,
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

// openFile opens a file for reading, and closes it when the test completes.
func openFile(t *testing.T, path string) io.Reader {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal("Failed to open file for reading:", err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error("Failed to close file:", err)
		}
	})

	return f
}

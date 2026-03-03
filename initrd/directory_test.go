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

	"github.com/unikraft/go-cpio"
	"kraftkit.sh/archive"
	"kraftkit.sh/initrd"
)

func TestNewFromDirectoryToCPIO(t *testing.T) {
	if err := archive.Unarchive("testdata/rootfs.tar.gz", "testdata/rootfs"); err != nil {
		t.Fatal("Unarchive:", err)
	}

	ctx := context.Background()

	ird, err := initrd.NewFromDirectory(
		ctx,
		"testdata/rootfs",
		initrd.WithArchitecture("x86_64"),
		initrd.WithOutputType(initrd.FsTypeCpio),
	)
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
		if err := os.RemoveAll("testdata/rootfs"); err != nil {
			t.Fatal("Failed to remove rootfs directory:", err)
		}
	})

	r := cpio.NewReader(openFile(t, irdPath))

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
		// Special exception for the hardlink which has size of 13 on disk.
		if hdr.Size != expectHdr.Size && hdr.Name != "./a/b/c/f-hardlink" {
			t.Errorf("file [%s]: got size %d, expected %d", hdr.Name, hdr.Size, expectHdr.Size)
		}
	}
}

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

func TestNewFromDockerfile(t *testing.T) {
	const rootfsDockerfile = "testdata/rootfs.Dockerfile"

	ctx := context.Background()

	ird, err := initrd.NewFromDockerfile(ctx, rootfsDockerfile)
	if err != nil {
		t.Fatal("NewFromDockerfile:", err)
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

	r := cpio.NewReader(openFile(t, irdPath))

	expectHeaders := map[string]cpio.Header{
		"/a": {
			Mode: cpio.TypeDir,
		},
		"/a/b": {
			Mode: cpio.TypeDir,
		},
		"/a/b/c": {
			Mode: cpio.TypeDir,
		},
		"/a/b/c/d": {
			Mode: cpio.TypeReg,
			Size: 13,
		},
		"/a/b/c/e-symlink": {
			Mode:     cpio.TypeSymlink,
			Linkname: "./d",
		},
		"/a/b/c/f-hardlink": {
			Mode: cpio.TypeReg,
			Size: 0,
		},
		"/a/b/c/g-recursive-symlink": {
			Mode:     cpio.TypeSymlink,
			Linkname: ".",
		},
	}

	var gotFiles []string

	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal("Failed to read next cpio header:", err)
		}

		gotFiles = append(gotFiles, hdr.Name)

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

	if len(gotFiles) != len(expectHeaders) {
		t.Errorf("Expected %d files, got %d: %#v", len(expectHeaders), len(gotFiles), gotFiles)
	}
}

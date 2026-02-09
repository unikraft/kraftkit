// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package initrd_test

import (
	"io"
	"os"
	"testing"

	"github.com/unikraft/go-cpio"
)

var expectHeaders = map[string]cpio.Header{
	"./a": {
		Mode: cpio.TypeDir,
	},
	"./a/b": {
		Mode: cpio.TypeDir,
	},
	"./a/b/c": {
		Mode: cpio.TypeDir,
	},
	"./a/b/c/d": {
		Mode: cpio.TypeRegular,
		Size: 13,
	},
	"./a/b/c/e-symlink": {
		Mode:     cpio.TypeSymlink,
		Linkname: "./d",
	},
	"./a/b/c/f-hardlink": {
		Mode: cpio.TypeRegular,
		Size: 0,
	},
	"./a/b/c/g-recursive-symlink": {
		Mode:     cpio.TypeSymlink,
		Linkname: ".",
	},
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

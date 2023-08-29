// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"context"
	"io"
	"os"

	"github.com/cavaliergopher/cpio"
)

type file struct {
	opts  InitrdOptions
	path  string
	files []string
}

// NewFromFile accepts an input file which already represents a CPIO archive and
// is provided as a mechanism for satisfying the Initrd interface.
func NewFromFile(_ context.Context, path string, opts ...InitrdOption) (Initrd, error) {
	fi, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer fi.Close()

	initrd := file{
		opts: InitrdOptions{},
		path: path,
	}

	for _, opt := range opts {
		if err := opt(&initrd.opts); err != nil {
			return nil, err
		}
	}

	reader := cpio.NewReader(fi)

	// Iterate through the files in the archive.
	for {
		hdr, err := reader.Next()
		if err == io.EOF {
			// end of cpio archive
			break
		}
		if err != nil {
			return nil, err
		}

		initrd.files = append(initrd.files, hdr.Name)
	}

	return &initrd, nil
}

// Build implements Initrd.
func (initrd *file) Build(_ context.Context) (string, error) {
	return initrd.path, nil
}

// Files implements Initrd.
func (initrd *file) Files() []string {
	return initrd.files
}

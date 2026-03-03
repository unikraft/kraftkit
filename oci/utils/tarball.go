// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package utils

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
)

// OpenFromTarOCILayer returns a reader for the given file from the provided
// tarball reader.
func OpenFromTarOCILayer(r io.Reader, path string) (io.Reader, error) {
	tarPath, err := filepath.Rel("/", path)
	if err != nil {
		return nil, fmt.Errorf("could not trim leading separator from path %q: %w", path, err)
	}

	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("could not advance to the next entry in the OCI image layer: %w", err)
		}

		if h.Name != tarPath {
			continue
		}

		if t := h.Typeflag; t != tar.TypeReg {
			return nil, fmt.Errorf("path is not a regular file")
		}

		return tr, nil
	}

	return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
}

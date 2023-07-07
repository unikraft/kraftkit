// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package archive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/juju/errors"
)

// Unarchive takes an input src file and determines (based on its extension)
func Unarchive(src, dst string, opts ...UnarchiveOption) error {
	switch true {
	case strings.HasSuffix(src, ".tar.gz"):
		return UntarGz(src, dst, opts...)
	}

	return errors.Errorf("unrecognized extension: %s", filepath.Base(src))
}

// UntarGz unarchives a tarball which has been gzip compressed
func UntarGz(src, dst string, opts ...UnarchiveOption) error {
	f, err := os.Open(src)
	if err != nil {
		return errors.Annotate(err, "could not open file")
	}

	defer f.Close()

	gzipReader, err := gzip.NewReader(f)
	if err != nil {
		return errors.Annotate(err, "could not open gzip reader")
	}

	return Untar(gzipReader, dst, opts...)
}

// Untar unarchives a tarball which has been gzip compressed
func Untar(src io.Reader, dst string, opts ...UnarchiveOption) error {
	uc := &UnarchiveOptions{}
	for _, opt := range opts {
		if err := opt(uc); err != nil {
			return err
		}
	}

	tr := tar.NewReader(src)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		var path string
		if uc.stripComponents > 0 {
			// We don't use the context-(host-)specific filepath.SplitList because
			// this is a UNIX tarball
			parts := strings.Split(header.Name, "/")
			path = strings.Join(parts[uc.stripComponents:], "/")
			path = filepath.Join(dst, path)
		} else {
			path = filepath.Join(dst, header.Name)
		}

		info := header.FileInfo()

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, info.Mode()); err != nil {
				return errors.Annotate(err, "could not create directory")
			}

		case tar.TypeReg:
			// Create parent path if it does not exist
			if err := os.MkdirAll(filepath.Dir(path), info.Mode()); err != nil {
				return errors.Annotate(err, "could not create directory")
			}

			newFile, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
			if err != nil {
				return errors.Annotate(err, "could not create file")
			}

			if _, err := io.Copy(newFile, tr); err != nil {
				newFile.Close()
				return errors.Annotate(err, "could not copy file")
			}

			newFile.Close()

			// TODO: Are there any other files we should consider?
			// default:
			// 	return fmt.Errorf("unknown type: %s in %s", string(header.Typeflag), path)
		}

		// Change access time and modification time if possible (error ignored)
		_ = os.Chtimes(path, header.AccessTime, header.ModTime)
	}

	return nil
}

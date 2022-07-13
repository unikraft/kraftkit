// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Unarchive takes an input src file and determines (based on its extension)
func Unarchive(src, dst string, opts ...UnarchiveOption) error {
	switch true {
	case strings.HasSuffix(src, ".tar.gz"):
		return UntarGz(src, dst, opts...)
	}

	return fmt.Errorf("unrecognized extension: %s", filepath.Base(src))
}

// UntarGz unarchives a tarball which has been gzip compressed
func UntarGz(src, dst string, opts ...UnarchiveOption) error {
	uc := &UnarchiveOptions{}
	for _, opt := range opts {
		if err := opt(uc); err != nil {
			return err
		}
	}

	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("could not open file: %v", err)
	}

	defer f.Close()

	gzipReader, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("could not open gzip reader: %v", err)
	}

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}
		
		var path string
		if uc.StripComponents > 0 {
			// We don't use the context-(host-)specific filepath.SplitList because
			// this is a UNIX tarball
			parts := strings.Split(header.Name, "/")
			path = strings.Join(parts[uc.StripComponents:], "/")
			path = filepath.Join(dst, path)
		} else {
			path = filepath.Join(dst, header.Name)
		}

		info := header.FileInfo()

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, info.Mode()); err != nil {
				return fmt.Errorf("could not create directory: %v", err)
			}

		case tar.TypeReg:
			newFile, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
			if err != nil {
				return fmt.Errorf("could not create file: %v", err)
			}

			if _, err := io.Copy(newFile, tarReader); err != nil {
				newFile.Close()
				return fmt.Errorf("could not copy file: %v", err)
			}

			newFile.Close()
		
		// TODO: Are there any other files we should consider?
		// default:
		// 	return fmt.Errorf("unknown type: %s in %s", string(header.Typeflag), path)
		}
	}

	return nil
}

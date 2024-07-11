// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2017, Ryan Armstrong.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cpio

import (
	"io"
)

// Reader provides sequential access to the contents of a CPIO archive.
// Reader.Next advances to the next file in the archive (including the first),
// and then Reader can be treated as an io.Reader to access the file's data.
type Reader struct {
	r   io.Reader // underlying file reader
	hdr *Header   // current Header
	eof int64     // bytes until the end of the current file
}

// NewReader creates a new Reader reading from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		r: r,
	}
}

// Read reads from the current file in the CPIO archive. It returns (0, io.EOF)
// when it reaches the end of that file, until Next is called to advance to the
// next file.
//
// Calling Read on special types like TypeLink, TypeSymlink, TypeChar,
// TypeBlock, TypeDir, and TypeFifo returns (0, io.EOF) regardless of what the
// Header.Size claims.
func (r *Reader) Read(p []byte) (n int, err error) {
	if r.hdr == nil || r.eof == 0 {
		return 0, io.EOF
	}
	rn := len(p)
	if r.eof < int64(rn) {
		rn = int(r.eof)
	}
	n, err = r.r.Read(p[0:rn])
	r.eof -= int64(n)
	return
}

// Next advances to the next entry in the CPIO archive. The Header.Size
// determines how many bytes can be read for the next file. Any remaining data
// in the current file is automatically discarded.
//
// io.EOF is returned at the end of the input.
func (r *Reader) Next() (*Header, *RawHeader, error) {
	if r.hdr == nil {
		return r.next()
	}
	skp := r.eof + r.hdr.EntryPad
	if skp > 0 {
		_, err := io.CopyN(io.Discard, r.r, skp)
		if err != nil {
			return nil, nil, err
		}
	}
	return r.next()
}

func (r *Reader) next() (*Header, *RawHeader, error) {
	r.eof = 0
	hdr, raw, err := readSVR4Header(r.r)
	if err != nil {
		return hdr, raw, err
	}
	r.hdr = hdr
	r.eof = hdr.Size
	return hdr, raw, nil
}

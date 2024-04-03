// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package vimport

import (
	"io"
	"os"
)

// newFileProgress wraps the given os.File so that the number of bytes copied
// from it can be tracked an reported through the given callback function.
func newFileProgress(f *os.File, fsize int64, callback progressCallbackFunc) *fileProgress {
	return &fileProgress{
		f:        f,
		fsize:    fsize,
		callback: callback,
	}
}

type progressCallbackFunc func(progress float64)

// fileProgress can track the number of bytes copied from an os.File.
type fileProgress struct {
	f        *os.File
	fsize    int64
	cpBytes  int
	callback progressCallbackFunc
}

// NOTE(antoineco): fileProgress must not implement io.WriterTo, otherwise
// io.Copy favours that implementation over Read() and we lose the ability to
// track progress.
var _ io.Reader = (*fileProgress)(nil)

// Read implements io.Reader.
func (f *fileProgress) Read(b []byte) (int, error) {
	n, err := f.f.Read(b)

	f.cpBytes += n
	pct := float64(f.cpBytes) / float64(f.fsize)
	// FIXME(antoineco): the TUI component does not turn green at the end of the
	// copy if we call callback() with a value of 1.
	if pct < 1.0 {
		f.callback(pct)
	}

	return n, err
}

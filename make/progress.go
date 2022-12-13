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

package make

import (
	"io"
	"strings"
)

var IgnoredMakePrefixes = []string{
	"make[",
}

type onProgressWriter struct {
	onProgress func(float64)
	current    int
	total      int
	io.Writer
}

func (opw *onProgressWriter) Write(b []byte) (int, error) {
	// Remove the last newline character if present
	line := strings.TrimSuffix(string(b), "\n")

	// Split all lines up so we can individually count them
	lines := strings.Split(strings.ReplaceAll(line, "\r\n", "\n"), "\n")

	// Cound the total number of non-ignored lines
	for _, line := range lines {
		for _, ignore := range IgnoredMakePrefixes {
			if !strings.HasPrefix(line, ignore) {
				opw.current++
			}
		}
	}

	opw.onProgress(float64(opw.current) / float64(opw.total))

	return len(b), nil
}

type calculateProgressWriter struct {
	io.Writer
	totalLines int
}

func (cpw *calculateProgressWriter) Write(b []byte) (int, error) {
	// Remove the last newline character if present
	line := strings.TrimSuffix(string(b), "\n")

	// Split all lines up so we can individually count them
	lines := strings.Split(strings.ReplaceAll(line, "\r\n", "\n"), "\n")

	// Cound the total number of non-ignored lines
	for _, line := range lines {
		for _, ignore := range IgnoredMakePrefixes {
			if !strings.HasPrefix(line, ignore) {
				cpw.totalLines++
			}
		}
	}

	return len(b), nil
}

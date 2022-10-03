// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Cezar Craciunoiu <cezar.craciunoiu@gmail.com
//
// Copyright (c) 2022, Universitatea POLITEHNICA Bucharest.  All rights reserved.
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

package uiformatter

import (
	"fmt"
	"io"
	"strings"

	"kraftkit.sh/iostreams"
)

type StdType int

const (
	stderr = iota
	stdout
)

type StdLine struct {
	lineType StdType
	line     string
}

var (
	OutputLines = make([]StdLine, 0)
)

type FormatterWriters struct {
	Err OnErrorWriter
	Out OnOutputWriter
}

type OnErrorWriter struct {
	ErrWriter io.Writer
}

type OnOutputWriter struct {
	OutWriter io.Writer
}

func (oew *OnErrorWriter) Write(b []byte) (int, error) {

	// Split on newlines and add the lines to the OutputLines array
	for _, line := range strings.Split(string(b), "\n") {
		OutputLines = append(OutputLines, StdLine{lineType: stderr, line: line})
	}

	return len(b), nil
}

func (oew *OnErrorWriter) Flush() {
	// Print the errors to the terminal at the end of the build
	firstLine := true
	for _, line := range OutputLines {
		if line.lineType == stderr {
			if firstLine {
				line.line = AppendWhitespace(line.line)
				firstLine = false
			}

			// TODO - Possibly wrong to write with fmt
			// The builds are finished at this step so it should be fine
			fmt.Printf("%s\n", line.line)
		}
	}

	// TODO - probably wrong when building multiple images at the same time
	// If that is the case, we need to keep track of the jobs that the lines belong to
	OutputLines = make([]StdLine, 0)
}

func (oow *OnOutputWriter) Write(b []byte) (int, error) {
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		OutputLines = append(OutputLines, StdLine{lineType: stdout, line: line})
	}

	return len(b), nil
}

// Eliminate all lines that start with a prefix. Use a hard limit to stop.
// Returns the offset of lines that were removed.
func RemoveLines(prefix string, limit int) []int {
	var removed []int

	for i := len(OutputLines) - 1; i >= 0; i-- {
		if OutputLines[i].lineType == stdout && strings.HasPrefix(OutputLines[i].line, prefix) {
			removed = append(removed, len(OutputLines)-1-i)
			OutputLines = append(OutputLines[:i], OutputLines[i+1:]...)
		}
		if len(removed) >= limit {
			break
		}
	}

	return removed
}

// Appends whitespace to fill the size of the terminal
func AppendWhitespace(message string) string {
	widthSize := iostreams.System().TerminalWidth()
	percentageMaxSize := 5

	if len(message) < widthSize {
		message = message + strings.Repeat(" ", widthSize-len(message)+percentageMaxSize)
	}

	return message
}

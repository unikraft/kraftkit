// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	gomegafmt "github.com/onsi/gomega/format"
)

// NewKraft returns a kraft OS command that uses the given IO streams and has
// pre-set flags to use the given paths.
func NewKraft(stdout, stderr *IOStream) *exec.Cmd {
	cmd := exec.Command("kraft")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd
}

// IOStream represents an IO stream to be used by OS commands and suitable
// for assertions and reporting in tests.
type IOStream struct {
	b *bytes.Buffer
}

var (
	_ io.ReadWriter            = (*IOStream)(nil)
	_ fmt.Stringer             = (*IOStream)(nil)
	_ gomegafmt.GomegaStringer = (*IOStream)(nil)
)

// NewIOStream returns an initialized IOStream.
func NewIOStream() *IOStream {
	return &IOStream{
		b: &bytes.Buffer{},
	}
}

func (s *IOStream) Read(p []byte) (n int, err error) {
	return s.b.Read(p)
}

func (s *IOStream) Write(p []byte) (n int, err error) {
	return s.b.Write(p)
}

func (s *IOStream) String() string {
	return s.b.String()
}

func (s *IOStream) GomegaString() string {
	return s.String()
}

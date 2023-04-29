// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"

	gomegafmt "github.com/onsi/gomega/format"
)

// NewKraft returns a kraft OS command that uses the given IO streams and has
// pre-set flags to use the given paths.
func NewKraft(stdout, stderr *IOStream, cfgPath string) *Cmd {
	var args []string
	if cfgPath != "" {
		args = append(args, "--config-dir="+filepath.Dir(cfgPath))
	}

	cmd := exec.Command("kraft", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return &Cmd{Cmd: cmd}
}

// Cmd is a wrapper around exec.Cmd with sensible handling of stderr in error
// reports.
type Cmd struct {
	*exec.Cmd
}

// Run runs the command, and automatically injects the output to stderr in the
// returned ExitError, in case such an error occurs.
// It is similar to (*exec.Cmd).Output, but allows the command to have stdout
// explicitly set.
func (c *Cmd) Run() error {
	if err := c.Cmd.Run(); err != nil {
		if ee := (&exec.ExitError{}); errors.As(err, &ee) {
			if r, ok := c.Cmd.Stderr.(io.Reader); ok {
				b, re := io.ReadAll(r)
				if re != nil {
					return fmt.Errorf("%w. Additionally, while reading stderr: %w", err, re)
				}
				ee.Stderr = b
				return &ExitError{ExitError: ee}
			}
		}
	}

	return nil
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

// ExitError is a wrapper around exec.ExitError that can be pretty-printed
// through a gomega matcher.
type ExitError struct {
	*exec.ExitError
}

var (
	_ error                    = (*ExitError)(nil)
	_ gomegafmt.GomegaStringer = (*IOStream)(nil)
)

func (e *ExitError) GomegaString() string {
	if len(e.ExitError.Stderr) > 0 {
		return string(e.ExitError.Stderr)
	}
	return ""
}

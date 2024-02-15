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
	"os/user"
	"path/filepath"
	"strings"

	gomegafmt "github.com/onsi/gomega/format"
)

func NewCurl(stdout, stderr *IOStream) *Cmd {
	cmd := exec.Command("curl")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	cmd.Args = append(cmd.Args, "-s", "-S", "-L", "--fail-with-body")

	return &Cmd{Cmd: cmd}
}

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

// NewKraftPrivileged returns a kraft OS command that uses the given IO streams and has
// pre-set flags to use the given paths, with a sudo prefix if user is not already root.
func NewKraftPrivileged(stdout, stderr *IOStream, cfgPath string) *Cmd {
	var args []string
	// Get uid of current user
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}

	if usr.Uid != "0" {
		kraftPath, _ := exec.LookPath("kraft")

		args = append(args, kraftPath)
	}

	if cfgPath != "" {
		args = append(args, "--config-dir="+filepath.Dir(cfgPath))
	}

	var cmd *exec.Cmd
	if usr.Uid != "0" {
		cmd = exec.Command("sudo", args...)
	} else {
		cmd = exec.Command("kraft", args...)
	}
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

// DumpError is a common method used across command executions which is
// used to standardize the output display of the command which was invoked
// along with the stdout and stderr.
func (c *Cmd) DumpError(stdoutio, stderrio *IOStream, err error) string {
	var builder strings.Builder

	builder.WriteString("\n")
	builder.WriteString(strings.Join(c.Args, " "))
	builder.WriteString("\n")

	stdout := strings.Split(stdoutio.String(), "\n")
	if !(len(stdout) == 1 && stdout[0] == "") {
		for _, line := range stdout {
			builder.WriteString("stdout > ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n")

	stderr := strings.Split(stderrio.String(), "\n")
	if !(len(stdout) == 1 && stdout[0] == "") {
		for _, line := range stderr {
			builder.WriteString("stderr > ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}

	return builder.String()
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

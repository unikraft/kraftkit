// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package exec

import (
	"fmt"
	"io"
)

type ExecOptions struct {
	stderr    io.Writer
	stdout    io.Writer
	stderrcbs []io.Writer
	stdoutcbs []io.Writer
	stdin     io.Reader
	env       []string
	callbacks []func(int)
	detach    bool
}

type ExecOption func(eo *ExecOptions) error

// NewExecOptions accepts a series of options and returns a rendered
// *ExecOptions structure
func NewExecOptions(eopts ...ExecOption) (*ExecOptions, error) {
	eo := &ExecOptions{}

	for _, o := range eopts {
		if err := o(eo); err != nil {
			return nil, fmt.Errorf("could not apply option: %v", err)
		}
	}

	return eo, nil
}

// WithEnvKey adds an additional environment by its key and value
func WithEnvKey(key, val string) ExecOption {
	return func(eo *ExecOptions) error {
		if eo.env == nil {
			eo.env = make([]string, 0)
		}

		eo.env = append(eo.env, fmt.Sprintf("%s=%s", key, val))

		return nil
	}
}

// WithOnExitCallback sets callback method where its only parameter is the exit
// code returned by the process.  This method can be called multiple times.
func WithOnExitCallback(callback func(int)) ExecOption {
	return func(eo *ExecOptions) error {
		if eo.callbacks == nil {
			eo.callbacks = make([]func(int), 0)
		}

		eo.callbacks = append(eo.callbacks, callback)

		return nil
	}
}

// WithStdout sets the primary stdout for the process
func WithStdout(stdout io.Writer) ExecOption {
	return func(eo *ExecOptions) error {
		eo.stdout = stdout
		return nil
	}
}

// WithStderr sets the primary stderr for the process
func WithStderr(stderr io.Writer) ExecOption {
	return func(eo *ExecOptions) error {
		eo.stderr = stderr
		return nil
	}
}

// WithStdin sets the primary stdin for the process
func WithStdin(stdin io.Reader) ExecOption {
	return func(eo *ExecOptions) error {
		eo.stdin = stdin
		return nil
	}
}

// WithStdoutCallback adds a callback which will be
func WithStdoutCallback(stdoutcb io.Writer) ExecOption {
	return func(eo *ExecOptions) error {
		if eo.stdoutcbs == nil {
			eo.stdoutcbs = make([]io.Writer, 0)
		}

		eo.stdoutcbs = append(eo.stdoutcbs, stdoutcb)

		return nil
	}
}

func WithStderrCallback(stderrcb io.Writer) ExecOption {
	return func(eo *ExecOptions) error {
		if eo.stderrcbs == nil {
			eo.stderrcbs = make([]io.Writer, 0)
		}

		eo.stderrcbs = append(eo.stderrcbs, stderrcb)

		return nil
	}
}

func WithDetach(detach bool) ExecOption {
	return func(eo *ExecOptions) error {
		eo.detach = detach
		return nil
	}
}

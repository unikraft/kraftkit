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

package exec

import (
	"context"
	"fmt"
	"io"

	"kraftkit.sh/log"
)

type ExecOptions struct {
	ctx       context.Context
	stderr    io.Writer
	stdout    io.Writer
	stderrcbs []io.Writer
	stdoutcbs []io.Writer
	stdin     io.Reader
	env       []string
	log       log.Logger
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

// WithContext sets the context for the process
func WithContext(ctx context.Context) ExecOption {
	return func(eo *ExecOptions) error {
		eo.ctx = ctx
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

func WithLogger(l log.Logger) ExecOption {
	return func(eo *ExecOptions) error {
		eo.log = l
		return nil
	}
}

func WithDetach(detach bool) ExecOption {
	return func(eo *ExecOptions) error {
		eo.detach = detach
		return nil
	}
}

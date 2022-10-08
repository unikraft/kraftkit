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
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type Process struct {
	executable *Executable
	opts       *ExecOptions
	cmd        *exec.Cmd
}

// NewProcess prepares a process to be executed from a given binary name and
// optional execution options
func NewProcess(bin string, args []string, eopts ...ExecOption) (*Process, error) {
	executable, err := NewExecutable(bin, nil)
	if err != nil {
		return nil, err
	}

	executable.args = append(executable.args, args...)

	return NewProcessFromExecutable(executable, eopts...)
}

// NewProcessFromExecutable prepares a process to be executed from a given
// *Executable object and optional execution options
func NewProcessFromExecutable(executable *Executable, eopts ...ExecOption) (*Process, error) {
	if executable == nil {
		return nil, fmt.Errorf("cannot prepare process without executable")
	}

	opts, err := NewExecOptions(eopts...)
	if err != nil {
		return nil, err
	}

	e := &Process{
		executable: executable,
		opts:       opts,
	}

	return e, nil
}

// Cmdline returns the full command line to be executed
func (e *Process) Cmdline() string {
	return strings.Join(
		append(
			[]string{e.executable.bin},
			e.executable.Args()...,
		),
		" ",
	)
}

// Start the process
func (e *Process) Start() error {
	if e.opts.ctx != nil {
		e.cmd = exec.CommandContext(
			e.opts.ctx,
			e.executable.bin,
			e.executable.Args()...,
		)
	} else {
		e.cmd = exec.Command(e.executable.bin, e.executable.Args()...)
	}

	// Set the stdout
	if e.opts.stdout != nil && len(e.opts.stdoutcbs) == 0 {
		e.cmd.Stdout = e.opts.stdout
	} else if e.opts.stdout != nil && len(e.opts.stdoutcbs) > 0 {
		e.cmd.Stdout = io.MultiWriter(
			append([]io.Writer{e.opts.stdout}, e.opts.stdoutcbs...)...,
		)
	} else if len(e.opts.stdoutcbs) > 0 {
		e.cmd.Stdout = io.MultiWriter(e.opts.stdoutcbs...)
	}

	// Set the stderr
	if e.opts.stderr != nil && len(e.opts.stderrcbs) == 0 {
		e.cmd.Stderr = e.opts.stderr
	} else if e.opts.stderr != nil && len(e.opts.stderrcbs) > 0 {
		e.cmd.Stderr = io.MultiWriter(
			append([]io.Writer{e.opts.stderr}, e.opts.stderrcbs...)...,
		)
	} else if e.opts.stdout != nil && len(e.opts.stderrcbs) == 0 {
		e.cmd.Stderr = e.opts.stdout
	} else if e.opts.stdout != nil && len(e.opts.stderrcbs) > 0 {
		e.cmd.Stderr = io.MultiWriter(
			append([]io.Writer{e.opts.stdout}, e.opts.stderrcbs...)...,
		)
	} else if len(e.opts.stderrcbs) > 0 {
		e.cmd.Stderr = io.MultiWriter(e.opts.stderrcbs...)
	}

	// Set the stdin
	if e.opts.stdin != nil {
		e.cmd.Stdin = e.opts.stdin
	}

	// Add any set environmental variables including the host's
	e.cmd.Env = append(os.Environ(), e.opts.env...)

	if e.opts.log != nil {
		e.opts.log.Debug(e.Cmdline())
	}

	return e.cmd.Start()
}

// Wait for the process to complete
func (e *Process) Wait() error {
	if e.cmd == nil {
		return fmt.Errorf("process has not yet started cannot wait")
	}

	err := e.cmd.Wait()
	if len(e.opts.callbacks) > 0 {
		for _, cb := range e.opts.callbacks {
			cb(e.cmd.ProcessState.ExitCode())
		}
	}

	return err
}

// StartAndWait starts the process and waits for it to exit
func (e *Process) StartAndWait() error {
	if err := e.Start(); err != nil {
		return err
	}

	return e.Wait()
}

// Signal sends a signal to the running process.  If this fails, for example if
// the process is not running, this will return an error.
func (e *Process) Signal(signal syscall.Signal) error {
	return e.cmd.Process.Signal(signal)
}

// Kill sends a SIGKILL to the running process.  If this fails, for example if
// the process is not running, this will return an error.
func (e *Process) Kill() error {
	return e.Signal(syscall.SIGKILL)
}

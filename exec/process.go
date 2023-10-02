// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package exec

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"kraftkit.sh/log"
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
func (e *Process) Start(ctx context.Context) error {
	e.cmd = exec.Command(
		e.executable.bin,
		e.executable.Args()...,
	)

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
	e.cmd.Stdin = e.opts.stdin

	// Add any set environmental variables including the host's
	e.cmd.Env = append(os.Environ(), e.opts.env...)

	log.G(ctx).Debug(e.Cmdline())

	if e.opts.detach {
		e.cmd.SysProcAttr = hostAttributes()
		e.cmd.Stdin = nil
	}

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("could not start process: %v", err)
	}

	return nil
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

// Release releases any resources associated with the process rendering it
// unusable in the future. Release only needs to be called if Wait is not.
func (e *Process) Release() error {
	return e.cmd.Process.Release()
}

// StartAndWait starts the process and waits for it to exit
func (e *Process) StartAndWait(ctx context.Context) error {
	if err := e.Start(ctx); err != nil {
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

// Pid returns the process ID
func (e *Process) Pid() (int, error) {
	if e.cmd == nil || e.cmd.Process == nil || e.cmd.Process.Pid == -1 {
		return -1, fmt.Errorf("could not locate pid")
	}

	return e.cmd.Process.Pid, nil
}

// Cmd returns the go Cmd of the process.
func (e *Process) Cmd() *exec.Cmd {
	return e.cmd
}

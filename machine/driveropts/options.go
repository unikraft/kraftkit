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

package driveropts

import (
	"os"

	"kraftkit.sh/config"
	"kraftkit.sh/exec"
	"kraftkit.sh/log"
	"kraftkit.sh/machine"
)

type DriverOptions struct {
	Log         log.Logger
	ExecOptions []exec.ExecOption
	Debug       bool
	RuntimeDir  string
	Background  bool
	Store       *machine.MachineStore
}

type DriverOption func(do *DriverOptions) error

func NewDriverOptions(opts ...DriverOption) (*DriverOptions, error) {
	dopts := DriverOptions{}

	for _, o := range opts {
		if err := o(&dopts); err != nil {
			return nil, err
		}
	}

	// Handle default values
	if len(dopts.RuntimeDir) == 0 {
		dopts.RuntimeDir = config.DefaultRuntimeDir
	}

	return &dopts, nil
}

func WithDebug(debug bool) DriverOption {
	return func(do *DriverOptions) error {
		do.Debug = debug
		return nil
	}
}

// WithRuntimeDir sets the location of files associated with the runtime of
// KraftKit.  This typically includes PID files, socket files, key-value
// databases, log files, etc.
func WithRuntimeDir(dir string) DriverOption {
	return func(do *DriverOptions) error {
		_, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		do.RuntimeDir = dir
		return nil
	}
}

// WithExecOptions offers configuration options to the underlying process
// executor
func WithExecOptions(eopts ...exec.ExecOption) DriverOption {
	return func(do *DriverOptions) error {
		if do.ExecOptions == nil {
			do.ExecOptions = make([]exec.ExecOption, 0)
		}

		do.ExecOptions = append(do.ExecOptions, eopts...)

		return nil
	}
}

// WithLogger provides access to a logger to be used within the package
func WithLogger(l log.Logger) DriverOption {
	return func(do *DriverOptions) error {
		do.Log = l
		do.ExecOptions = append(do.ExecOptions,
			exec.WithLogger(l),
		)
		return nil
	}
}

// WithBackground indicates as to whether the driver should start in the
// background
func WithBackground(background bool) DriverOption {
	return func(do *DriverOptions) error {
		do.Background = background
		return nil
	}
}

// WithMachineStore passes in an already instantiated `machine.MachineStore`
func WithMachineStore(store *machine.MachineStore) DriverOption {
	return func(do *DriverOptions) error {
		do.Store = store
		return nil
	}
}

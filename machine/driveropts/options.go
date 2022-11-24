// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
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

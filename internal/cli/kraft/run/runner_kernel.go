// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package run

import (
	"context"
	"debug/elf"
	"fmt"
	"path/filepath"

	machineapi "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
)

// runnerKernel is a simple runner used for instantiating a prebuilt Unikraft
// unikernel which is used in the most verbose usecase.  E.g.:
//
//	$ kraft run path/to/kernel_qemu-x86_64
type runnerKernel struct {
	kernelPath string
	args       []string
}

// String implements Runner.
func (runner *runnerKernel) String() string {
	return "kernel"
}

// Runnable implements Runner.
func (runner *runnerKernel) Runnable(ctx context.Context, opts *RunOptions, args ...string) (bool, error) {
	if len(args) == 0 {
		return false, fmt.Errorf("no arguments supplied")
	}

	var err error
	runner.kernelPath, err = filepath.Abs(args[0])
	if err != nil {
		return false, err
	}

	runner.args = args[1:]
	return unikraft.IsFileUnikraftUnikernel(runner.kernelPath)
}

// Prepare implements Runner.
func (runner *runnerKernel) Prepare(ctx context.Context, opts *RunOptions, machine *machineapi.Machine, args ...string) error {
	filename := filepath.Base(runner.kernelPath)
	machine.Spec.Platform = opts.platform.String()
	machine.Spec.Kernel = "kernel://" + filename
	machine.Status.KernelPath = runner.kernelPath
	machine.Spec.ApplicationArgs = runner.args

	// We need to know the architecture pre-emptively, see if we can
	// "intelligently" guess this by inspecting the ELF binary if the -m|--arch
	// has not been provided.
	if opts.Architecture == "" {
		fe, err := elf.Open(runner.kernelPath)
		if err != nil {
			return err
		}

		defer fe.Close()

		switch fe.Machine {
		case elf.EM_X86_64, elf.EM_386:
			machine.Spec.Architecture = arch.ArchitectureX86_64.String()
		case elf.EM_ARM:
			machine.Spec.Architecture = arch.ArchitectureArm.String()
		case elf.EM_AARCH64:
			machine.Spec.Architecture = arch.ArchitectureArm64.String()
		default:
			return fmt.Errorf("unsupported kernel architecture: %v", fe.Machine.String())
		}
	} else {
		machine.Spec.Architecture = opts.Architecture
	}

	return nil
}

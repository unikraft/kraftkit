// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

import "kraftkit.sh/exec"

// MachineServiceV1alpha1Option represents an option-method handler for the
// machinev1alpha1 service.
type MachineServiceV1alpha1Option func(*machineV1alpha1Service) error

// WithExecOptions passes additional kraftkit.sh/exec options to any sub-process
// invocation called within the machine service.
func WithExecOptions(eopts ...exec.ExecOption) MachineServiceV1alpha1Option {
	return func(service *machineV1alpha1Service) error {
		service.eopts = eopts
		return nil
	}
}

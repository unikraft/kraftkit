// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package firecracker

import "time"

// MachineServiceV1alpha1Option represents an option-method handler for the
// machinev1alpha1 service.
type MachineServiceV1alpha1Option func(*machineV1alpha1Service) error

// WithTimeout sets the time out when communicating with the firecracker socket
// API.
func WithTimeout(timeout time.Duration) MachineServiceV1alpha1Option {
	return func(service *machineV1alpha1Service) error {
		service.timeout = timeout
		return nil
	}
}

// WithDebug enables firecracker's internal debugging.
func WithDebug(debug bool) MachineServiceV1alpha1Option {
	return func(service *machineV1alpha1Service) error {
		service.debug = debug
		return nil
	}
}

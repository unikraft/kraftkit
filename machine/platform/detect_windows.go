// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package platform

import (
	"context"
	"fmt"
)

type SystemMode string

const (
	SystemUnknown = SystemMode("unknown")
	SystemGuest   = SystemMode("guest")
	SystemHost    = SystemMode("host")
)

// Detect returns the hypervisor and system mode in the context to the
// determined hypervisor or an error if not detectable.
func Detect(ctx context.Context) (Platform, SystemMode, error) {
	return PlatformUnknown, SystemUnknown, fmt.Errorf("Hypervisor detection is not supported on Windows")
}

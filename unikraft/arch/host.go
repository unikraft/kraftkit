// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package arch

import (
	"fmt"
	"runtime"
)

// HostArchitecture returns the architecture of the host or an error if
// unsupported by Unikraft.
func HostArchitecture() (string, error) {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		return "x86_64", nil
	case "arm", "arm64":
		return arch, nil
	default:
		return "", fmt.Errorf("unsupported architecture: %v", arch)
	}
}

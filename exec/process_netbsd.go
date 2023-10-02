// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package exec

import (
	"syscall"
)

func hostAttributes() *syscall.SysProcAttr {
	// the Setpgid flag is used to prevent the child process from exiting when
	// the parent is killed
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

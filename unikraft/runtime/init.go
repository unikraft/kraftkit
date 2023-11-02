// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package runtime

import "kraftkit.sh/cmdfactory"

var defaultRuntime string

func RegisterFlags() {
	// Register additional command-line arguments
	cmdfactory.RegisterFlag(
		"kraft run",
		cmdfactory.StringVar(
			&defaultRuntime,
			"runtime",
			defaultRuntime,
			"Set the path to an alternative ELF loader unikernel.",
		),
	)
}

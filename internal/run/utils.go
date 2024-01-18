// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package run

import "strings"

// BootArgsPrepare prepares the boot arguments for the unikernel.
// Because of double parsing we need to be careful to escape each argument
// with either single or double quotes before merging them together.
func BootArgsPrepare(args ...string) string {
	var sb strings.Builder
	for _, arg := range args {
		if strings.Count(arg, "'") != strings.Count(arg, "\\'") {
			sb.WriteByte('"')
			sb.Write([]byte(arg))
			sb.WriteByte('"')
			sb.WriteByte(' ')
		} else {
			sb.Write([]byte(arg))
			sb.WriteByte(' ')
		}
	}

	return sb.String()
}

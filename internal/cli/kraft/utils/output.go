// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2026, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

// IsValidOutputFormat returns true if the output format is supported.
func IsValidOutputFormat(format string) bool {
	return format == "json" ||
		format == "table" ||
		format == "yaml" ||
		format == "list" ||
		format == "raw" ||
		format == ""
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package version

import (
	"fmt"
)

var (
	version   = "No version provided"
	commit    = "No commit provided"
	buildTime = "No build timestamp provided"
)

// Version ...
func Version() string {
	return version
}

// Commit ...
func Commit() string {
	return commit
}

// BuildTime ...
func BuildTime() string {
	return buildTime
}

// String ...
func String() string {
	return fmt.Sprintf("%s (%s) built %s\n",
		version,
		commit,
		buildTime,
	)
}

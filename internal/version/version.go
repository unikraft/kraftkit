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

// Version returns KraftKit's version string.
func Version() string {
	return version
}

// Commit return KraftKit's HEAD Git commit SHA.
func Commit() string {
	return commit
}

// BuildTime returns the time in which the package or binary was built.
func BuildTime() string {
	return buildTime
}

// String returns all version information.
func String() string {
	return fmt.Sprintf("%s (%s) built %s\n",
		version,
		commit,
		buildTime,
	)
}

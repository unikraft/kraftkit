// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package main

import (
	"os"
	"path/filepath"

	"kraftkit.sh/internal/cli/runu"
)

func main() {
	// Make args[0] just the name of the executable since it is used in logs.
	os.Args[0] = filepath.Base(os.Args[0])

	os.Exit(runu.Main(os.Args))
}

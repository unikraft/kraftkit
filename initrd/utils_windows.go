//go:build windows
// +build windows

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"io/fs"

	"github.com/cavaliergopher/cpio"
)

func populateCPIO(info fs.FileInfo, header *cpio.Header) {
}

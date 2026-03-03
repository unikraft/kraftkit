//go:build !windows
// +build !windows

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package cpio

import (
	"io/fs"
	"syscall"

	"github.com/unikraft/go-cpio"
)

func FileInfoToCPIOHeader(info fs.FileInfo, header *cpio.Header) {
	if sysInfo := info.Sys(); sysInfo != nil {
		if stat, ok := sysInfo.(*syscall.Stat_t); ok {
			header.Uid = int(stat.Uid)
			header.Guid = int(stat.Gid)
			header.Inode = int64(stat.Ino)
			header.Links = int(stat.Nlink)
			header.DeviceID = int(stat.Dev)
		}
	}
}

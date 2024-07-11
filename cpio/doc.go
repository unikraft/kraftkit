// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2017, Ryan Armstrong.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

/*
Package cpio providers readers and writers for CPIO archives. Currently, only
the SVR4 (New ASCII) format is supported, both with and without checksums.

This package aims to be feel like Go's archive/tar package.

See the CPIO man page: https://www.freebsd.org/cgi/man.cgi?query=cpio&sektion=5
*/
package cpio

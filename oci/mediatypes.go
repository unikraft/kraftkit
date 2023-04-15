// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

const (
	MediaTypeLayer       = "application/vnd.unikraft.rootfs.diff"
	MediaTypeImageKernel = "application/vnd.unikraft.image.v1"
	MediaTypeInitrdCpio  = "application/vnd.unikraft.initrd.v1"
	MediaTypeConfig      = "application/vnd.unikraft.config.v1"

	MediaTypeLayerGzip       = MediaTypeLayer + "+gzip"
	MediaTypeImageKernelGzip = MediaTypeImageKernel + "+gzip"
	MediaTypeInitrdCpioGzip  = MediaTypeInitrdCpio + "+gzip"
	MediaTypeConfigGzip      = MediaTypeConfig + "+gzip"
)

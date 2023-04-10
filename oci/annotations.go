// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

const (
	AnnotationMediaType            = "org.unikraft.mediaType"
	AnnotationName                 = "org.unikraft.image.name"
	AnnotationVersion              = "org.unikraft.image.version"
	AnnotationURL                  = "org.unikraft.image.url"
	AnnotationCreated              = "org.unikraft.image.created"
	AnnotaitonDescription          = "org.unikraft.image.description"
	AnnotationKernelPath           = "org.unikraft.kernel.image"
	AnnotationKernelVersion        = "org.unikraft.kernel.version"
	AnnotationKernelInitrdPath     = "org.unikraft.kernel.initrd"
	AnnotationKernelKConfig        = "org.unikraft.kernel.kconfig."
	AnnotationKernelArch           = "org.unikraft.kernel.arch"
	AnnotationKernelPlat           = "org.unikraft.kernel.plat"
	AnnotationFilesystemPath       = "org.unikraft.filesystem"
	AnnotationDiskIndexPathPattern = "org.unikraft.disk-%d"
	AnnotationKraftKitVersion      = "sh.kraftkit.version"
)

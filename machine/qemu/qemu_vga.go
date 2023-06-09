// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

type QemuVGA string

const (
	QemuVGAStd    = QemuVGA("std")
	QemuVGACirrus = QemuVGA("cirrus")
	QemuVGAVMWare = QemuVGA("vmware")
	QemuVGAQxl    = QemuVGA("qxl")
	QemuVGAXenFb  = QemuVGA("xenfb")
	QemuVGATCX    = QemuVGA("tcx")
	QemuVGACG3    = QemuVGA("cg3")
	QemuVGAVirtio = QemuVGA("virtio")
	QemuVGANone   = QemuVGA("none")
)

func (qa QemuVGA) String() string {
	return string(qa)
}

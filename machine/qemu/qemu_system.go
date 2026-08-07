// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

const (
	QemuSystemX86     = "qemu-system-x86_64"
	QemuSystemArm     = "qemu-system-arm"
	QemuSystemAarch64 = "qemu-system-aarch64"
	QemuSystemRiscv64 = "qemu-system-riscv64"
)

func GetAllQemuSystemBinaries(extra []string) []string {
	binaries := []string{
		QemuSystemX86,
		QemuSystemArm,
		QemuSystemAarch64,
		QemuSystemRiscv64,
	}
	binaries = append(binaries, extra...)
	return binaries
}

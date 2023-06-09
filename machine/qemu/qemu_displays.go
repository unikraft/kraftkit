// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

import "fmt"

type QemuDisplayType string

const (
	QemuDisplayTypeAtiVga          = QemuDisplayType("ati-vga")            // bus PCI
	QemuDisplayTypeBochsDisplay    = QemuDisplayType("bochs-display")      // bus PCI
	QemuDisplayTypeCirrusVga       = QemuDisplayType("cirrus-vga")         // bus PCI, desc "Cirrus CLGD 54xx VGA"
	QemuDisplayTypeIsaCirrusVga    = QemuDisplayType("isa-cirrus-vga")     // bus ISA
	QemuDisplayTypeIsaVga          = QemuDisplayType("isa-vga")            // bus ISA
	QemuDisplayTypeQxl             = QemuDisplayType("qxl")                // bus PCI, desc "Spice QXL GPU (secondary)"
	QemuDisplayTypeQxlVga          = QemuDisplayType("qxl-vga")            // bus PCI, desc "Spice QXL GPU (primary, vga compatible)"
	QemuDisplayTypeRamfb           = QemuDisplayType("ramfb")              // bus System, desc "ram framebuffer standalone device"
	QemuDisplayTypeSecondaryVga    = QemuDisplayType("secondary-vga")      // bus PCI
	QemuDisplayTypeSga             = QemuDisplayType("sga")                // bus ISA, desc "Serial Graphics Adapter"
	QemuDisplayTypeVGA             = QemuDisplayType("VGA")                // bus PCI
	QemuDisplayTypeVhostUserGpu    = QemuDisplayType("vhost-user-gpu")     // bus virtio-bus
	QemuDisplayTypeVhostUserGpuPci = QemuDisplayType("vhost-user-gpu-pci") // bus PCI
	QemuDisplayTypeVhostUserVga    = QemuDisplayType("vhost-user-vga")     // bus PCI
	QemuDisplayTypeVirtioGpuDevice = QemuDisplayType("virtio-gpu-device")  // bus virtio-bus
	QemuDisplayTypeVirtioGpuPci    = QemuDisplayType("virtio-gpu-pci")     // bus PCI, alias "virtio-gpu"
	QemuDisplayTypeVirtioVga       = QemuDisplayType("virtio-vga")         // bus PCI
	QemuDisplayTypeVmwareSvga      = QemuDisplayType("vmware-svga")        // bus PCI
	QemuDisplayTypeNone            = QemuDisplayType("none")
)

type QemuDisplay interface {
	fmt.Stringer
}

type QemuDisplayNone struct{}

func (qd QemuDisplayNone) String() string {
	return string(QemuDisplayTypeNone)
}

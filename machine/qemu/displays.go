// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

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

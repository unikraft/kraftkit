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

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kraftkit.sh/exec"
	"kraftkit.sh/internal/retrytimeout"
	"kraftkit.sh/machine"
	"kraftkit.sh/machine/driveropts"
	"kraftkit.sh/machine/qemu/qmp"
	qmpv1alpha "kraftkit.sh/machine/qemu/qmp/v1alpha"

	goprocess "github.com/shirou/gopsutil/v3/process"
)

const (
	QemuSystemX86     = "qemu-system-x86_64"
	QemuSystemArm     = "qemu-system-arm"
	QemuSystemAarch64 = "qemu-system-aarch64"
)

type QemuDriver struct {
	dopts *driveropts.DriverOptions
}

func init() {
	// Register only used supported interfaces later used for serialization.  To
	// include all will roughly increase the final binary size by +20MB.

	// Character devices
	// gob.Register(QemuCharDevNull{})
	// gob.Register(QemuCharDevSocketTCP{})
	// gob.Register(QemuCharDevSocketUnix{})
	// gob.Register(QemuCharDevUdp{})
	// gob.Register(QemuCharDevVirtualConsole{})
	// gob.Register(QemuCharDevRingBuf{})
	// gob.Register(QemuCharDevFile{})
	// gob.Register(QemuCharDevPipe{})
	// gob.Register(QemuCharDevPty{})
	// gob.Register(QemuCharDevStdio{})
	// gob.Register(QemuCharDevSerial{})
	// gob.Register(QemuCharDevTty{})
	// gob.Register(QemuCharDevParallel{})
	// gob.Register(QemuCharDevParport{})
	// gob.Register(QemuCharDevSpiceVMC{})
	// gob.Register(QemuCharDevSpicePort{})

	// Host character devices
	// gob.Register(QemuHostCharDevVirtualConsole{})
	// gob.Register(QemuHostCharDevPty{})
	gob.Register(QemuHostCharDevNone{})
	// gob.Register(QemuHostCharDevNull{})
	// gob.Register(QemuHostCharDevNamed{})
	// gob.Register(QemuHostCharDevTty{})
	// gob.Register(QemuHostCharDevFile{})
	// gob.Register(QemuHostCharDevStdio{})
	// gob.Register(QemuHostCharDevPipe{})
	// gob.Register(QemuHostCharDevUDP{})
	// gob.Register(QemuHostCharDevTCP{})
	// gob.Register(QemuHostCharDevTelnet{})
	// gob.Register(QemuHostCharDevWebsocket{})
	gob.Register(QemuHostCharDevUnix{})

	// CPU devices
	// gob.Register(QemuDevice486V1X8664Cpu{})
	// gob.Register(QemuDevice486X8664Cpu{})
	// gob.Register(QemuDeviceAthlonV1X8664Cpu{})
	// gob.Register(QemuDeviceAthlonX8664Cpu{})
	// gob.Register(QemuDeviceBaseX8664Cpu{})
	// gob.Register(QemuDeviceBroadwellIbrsX8664Cpu{})
	// gob.Register(QemuDeviceBroadwellNotsxIbrsX8664Cpu{})
	// gob.Register(QemuDeviceBroadwellNotsxX8664Cpu{})
	// gob.Register(QemuDeviceBroadwellV1X8664Cpu{})
	// gob.Register(QemuDeviceBroadwellV2X8664Cpu{})
	// gob.Register(QemuDeviceBroadwellV3X8664Cpu{})
	// gob.Register(QemuDeviceBroadwellV4X8664Cpu{})
	// gob.Register(QemuDeviceBroadwellX8664Cpu{})
	// gob.Register(QemuDeviceCascadelakeServerNotsxX8664Cpu{})
	// gob.Register(QemuDeviceCascadelakeServerV1X8664Cpu{})
	// gob.Register(QemuDeviceCascadelakeServerV2X8664Cpu{})
	// gob.Register(QemuDeviceCascadelakeServerV3X8664Cpu{})
	// gob.Register(QemuDeviceCascadelakeServerV4X8664Cpu{})
	// gob.Register(QemuDeviceCascadelakeServerX8664Cpu{})
	// gob.Register(QemuDeviceConroeV1X8664Cpu{})
	// gob.Register(QemuDeviceConroeX8664Cpu{})
	// gob.Register(QemuDeviceCooperlakeV1X8664Cpu{})
	// gob.Register(QemuDeviceCooperlakeX8664Cpu{})
	// gob.Register(QemuDeviceCore2duoV1X8664Cpu{})
	// gob.Register(QemuDeviceCore2duoX8664Cpu{})
	// gob.Register(QemuDeviceCoreduoV1X8664Cpu{})
	// gob.Register(QemuDeviceCoreduoX8664Cpu{})
	// gob.Register(QemuDeviceDenvertonV1X8664Cpu{})
	// gob.Register(QemuDeviceDenvertonV2X8664Cpu{})
	// gob.Register(QemuDeviceDenvertonX8664Cpu{})
	// gob.Register(QemuDeviceDhyanaV1X8664Cpu{})
	// gob.Register(QemuDeviceDhyanaX8664Cpu{})
	// gob.Register(QemuDeviceEpycIbpbX8664Cpu{})
	// gob.Register(QemuDeviceEpycRomeV1X8664Cpu{})
	// gob.Register(QemuDeviceEpycRomeX8664Cpu{})
	// gob.Register(QemuDeviceEpycV1X8664Cpu{})
	// gob.Register(QemuDeviceEpycV2X8664Cpu{})
	// gob.Register(QemuDeviceEpycV3X8664Cpu{})
	// gob.Register(QemuDeviceEpycX8664Cpu{})
	// gob.Register(QemuDeviceHaswellIbrsX8664Cpu{})
	// gob.Register(QemuDeviceHaswellNotsxIbrsX8664Cpu{})
	// gob.Register(QemuDeviceHaswellNotsxX8664Cpu{})
	// gob.Register(QemuDeviceHaswellV1X8664Cpu{})
	// gob.Register(QemuDeviceHaswellV2X8664Cpu{})
	// gob.Register(QemuDeviceHaswellV3X8664Cpu{})
	// gob.Register(QemuDeviceHaswellV4X8664Cpu{})
	// gob.Register(QemuDeviceHaswellX8664Cpu{})
	// gob.Register(QemuDeviceHostX8664Cpu{})
	// gob.Register(QemuDeviceIcelakeClientNotsxX8664Cpu{})
	// gob.Register(QemuDeviceIcelakeClientV1X8664Cpu{})
	// gob.Register(QemuDeviceIcelakeClientV2X8664Cpu{})
	// gob.Register(QemuDeviceIcelakeClientX8664Cpu{})
	// gob.Register(QemuDeviceIcelakeServerNotsxX8664Cpu{})
	// gob.Register(QemuDeviceIcelakeServerV1X8664Cpu{})
	// gob.Register(QemuDeviceIcelakeServerV2X8664Cpu{})
	// gob.Register(QemuDeviceIcelakeServerV3X8664Cpu{})
	// gob.Register(QemuDeviceIcelakeServerV4X8664Cpu{})
	// gob.Register(QemuDeviceIcelakeServerX8664Cpu{})
	// gob.Register(QemuDeviceIvybridgeIbrsX8664Cpu{})
	// gob.Register(QemuDeviceIvybridgeV1X8664Cpu{})
	// gob.Register(QemuDeviceIvybridgeV2X8664Cpu{})
	// gob.Register(QemuDeviceIvybridgeX8664Cpu{})
	// gob.Register(QemuDeviceKnightsmillV1X8664Cpu{})
	// gob.Register(QemuDeviceKnightsmillX8664Cpu{})
	// gob.Register(QemuDeviceKvm32V1X8664Cpu{})
	// gob.Register(QemuDeviceKvm32X8664Cpu{})
	// gob.Register(QemuDeviceKvm64V1X8664Cpu{})
	// gob.Register(QemuDeviceKvm64X8664Cpu{})
	// gob.Register(QemuDeviceMaxX8664Cpu{})
	// gob.Register(QemuDeviceN270V1X8664Cpu{})
	// gob.Register(QemuDeviceN270X8664Cpu{})
	// gob.Register(QemuDeviceNehalemIbrsX8664Cpu{})
	// gob.Register(QemuDeviceNehalemV1X8664Cpu{})
	// gob.Register(QemuDeviceNehalemV2X8664Cpu{})
	// gob.Register(QemuDeviceNehalemX8664Cpu{})
	// gob.Register(QemuDeviceOpteronG1V1X8664Cpu{})
	// gob.Register(QemuDeviceOpteronG1X8664Cpu{})
	// gob.Register(QemuDeviceOpteronG2V1X8664Cpu{})
	// gob.Register(QemuDeviceOpteronG2X8664Cpu{})
	// gob.Register(QemuDeviceOpteronG3V1X8664Cpu{})
	// gob.Register(QemuDeviceOpteronG3X8664Cpu{})
	// gob.Register(QemuDeviceOpteronG4V1X8664Cpu{})
	// gob.Register(QemuDeviceOpteronG4X8664Cpu{})
	// gob.Register(QemuDeviceOpteronG5V1X8664Cpu{})
	// gob.Register(QemuDeviceOpteronG5X8664Cpu{})
	// gob.Register(QemuDevicePenrynV1X8664Cpu{})
	// gob.Register(QemuDevicePenrynX8664Cpu{})
	// gob.Register(QemuDevicePentiumV1X8664Cpu{})
	// gob.Register(QemuDevicePentiumX8664Cpu{})
	// gob.Register(QemuDevicePentium2V1X8664Cpu{})
	// gob.Register(QemuDevicePentium2X8664Cpu{})
	// gob.Register(QemuDevicePentium3V1X8664Cpu{})
	// gob.Register(QemuDevicePentium3X8664Cpu{})
	// gob.Register(QemuDevicePhenomV1X8664Cpu{})
	// gob.Register(QemuDevicePhenomX8664Cpu{})
	// gob.Register(QemuDeviceQemu32V1X8664Cpu{})
	// gob.Register(QemuDeviceQemu32X8664Cpu{})
	// gob.Register(QemuDeviceQemu64V1X8664Cpu{})
	// gob.Register(QemuDeviceQemu64X8664Cpu{})
	// gob.Register(QemuDeviceSandybridgeIbrsX8664Cpu{})
	// gob.Register(QemuDeviceSandybridgeV1X8664Cpu{})
	// gob.Register(QemuDeviceSandybridgeV2X8664Cpu{})
	// gob.Register(QemuDeviceSandybridgeX8664Cpu{})
	// gob.Register(QemuDeviceSkylakeClientIbrsX8664Cpu{})
	// gob.Register(QemuDeviceSkylakeClientNotsxIbrsX8664Cpu{})
	// gob.Register(QemuDeviceSkylakeClientV1X8664Cpu{})
	// gob.Register(QemuDeviceSkylakeClientV2X8664Cpu{})
	// gob.Register(QemuDeviceSkylakeClientV3X8664Cpu{})
	// gob.Register(QemuDeviceSkylakeClientX8664Cpu{})
	// gob.Register(QemuDeviceSkylakeServerIbrsX8664Cpu{})
	// gob.Register(QemuDeviceSkylakeServerNotsxIbrsX8664Cpu{})
	// gob.Register(QemuDeviceSkylakeServerV1X8664Cpu{})
	// gob.Register(QemuDeviceSkylakeServerV2X8664Cpu{})
	// gob.Register(QemuDeviceSkylakeServerV3X8664Cpu{})
	// gob.Register(QemuDeviceSkylakeServerV4X8664Cpu{})
	// gob.Register(QemuDeviceSkylakeServerX8664Cpu{})
	// gob.Register(QemuDeviceSnowridgeV1X8664Cpu{})
	// gob.Register(QemuDeviceSnowridgeV2X8664Cpu{})
	// gob.Register(QemuDeviceSnowridgeX8664Cpu{})
	// gob.Register(QemuDeviceWestmereIbrsX8664Cpu{})
	// gob.Register(QemuDeviceWestmereV1X8664Cpu{})
	// gob.Register(QemuDeviceWestmereV2X8664Cpu{})
	// gob.Register(QemuDeviceWestmereX8664Cpu{})

	// Controller/Bridge/Hub devices
	// gob.Register(QemuDeviceI82801b11Bridge{})
	// gob.Register(QemuDeviceIgdPassthroughIsaBridge{})
	// gob.Register(QemuDeviceIoh3420{})
	// gob.Register(QemuDevicePciBridge{})
	// gob.Register(QemuDevicePciBridgeSeat{})
	// gob.Register(QemuDevicePciePciBridge{})
	// gob.Register(QemuDevicePcieRootPort{})
	// gob.Register(QemuDevicePxb{})
	// gob.Register(QemuDevicePxbPcie{})
	// gob.Register(QemuDeviceUsbHost{})
	// gob.Register(QemuDeviceUsbHub{})
	// gob.Register(QemuDeviceVfioPciIgdLpcBridge{})
	// gob.Register(QemuDeviceVmbusBridge{})
	// gob.Register(QemuDeviceX3130Upstream{})
	// gob.Register(QemuDeviceXio3130Downstream{})

	// Display devices
	// gob.Register(QemuDeviceAtiVga{})
	// gob.Register(QemuDeviceBochsDisplay{})
	// gob.Register(QemuDeviceCirrusVga{})
	// gob.Register(QemuDeviceIsaCirrusVga{})
	// gob.Register(QemuDeviceIsaVga{})
	// gob.Register(QemuDeviceQxl{})
	// gob.Register(QemuDeviceQxlVga{})
	// gob.Register(QemuDeviceRamfb{})
	// gob.Register(QemuDeviceSecondaryVga{})
	gob.Register(QemuDeviceSga{})
	// gob.Register(QemuDeviceVga{})
	// gob.Register(QemuDeviceVhostUserGpu{})
	// gob.Register(QemuDeviceVhostUserGpuPci{})
	// gob.Register(QemuDeviceVhostUserVga{})
	// gob.Register(QemuDeviceVirtioGpuDevice{})
	// gob.Register(QemuDeviceVirtioGpuPci{})
	// gob.Register(QemuDeviceVirtioVga{})
	// gob.Register(QemuDeviceVmwareSvga{})

	// Input devices
	// gob.Register(QemuDeviceCcidCardEmulated{})
	// gob.Register(QemuDeviceCcidCardPassthru{})
	// gob.Register(QemuDeviceI8042{})
	// gob.Register(QemuDeviceIpoctal232{})
	// gob.Register(QemuDeviceIsaParallel{})
	// gob.Register(QemuDeviceIsaSerial{})
	// gob.Register(QemuDevicePciSerial{})
	// gob.Register(QemuDevicePciSerial2x{})
	// gob.Register(QemuDevicePciSerial4x{})
	// gob.Register(QemuDeviceTpci200{})
	// gob.Register(QemuDeviceUsbBraille{})
	// gob.Register(QemuDeviceUsbCcid{})
	// gob.Register(QemuDeviceUsbKbd{})
	// gob.Register(QemuDeviceUsbMouse{})
	// gob.Register(QemuDeviceUsbSerial{})
	// gob.Register(QemuDeviceUsbTablet{})
	// gob.Register(QemuDeviceUsbWacomTablet{})
	// gob.Register(QemuDeviceVhostUserInput{})
	// gob.Register(QemuDeviceVhostUserInputPci{})
	// gob.Register(QemuDeviceVirtconsole{})
	// gob.Register(QemuDeviceVirtioInputHostDevice{})
	// gob.Register(QemuDeviceVirtioInputHostPci{})
	// gob.Register(QemuDeviceVirtioKeyboardDevice{})
	// gob.Register(QemuDeviceVirtioKeyboardPci{})
	// gob.Register(QemuDeviceVirtioMouseDevice{})
	// gob.Register(QemuDeviceVirtioMousePci{})
	// gob.Register(QemuDeviceVirtioSerialDevice{})
	// gob.Register(QemuDeviceVirtioSerialPci{})
	// gob.Register(QemuDeviceVirtioSerialPciNonTransitional{})
	// gob.Register(QemuDeviceVirtioSerialPciTransitional{})
	// gob.Register(QemuDeviceVirtioTabletDevice{})
	// gob.Register(QemuDeviceVirtioTabletPci{})
	// gob.Register(QemuDeviceVirtserialport{})

	// Misc devices
	// gob.Register(QemuDeviceAmdIommu{})
	// gob.Register(QemuDeviceCtucanPci{})
	// gob.Register(QemuDeviceEdu{})
	// gob.Register(QemuDeviceHypervTestdev{})
	// gob.Register(QemuDeviceI2cDdc{})
	// gob.Register(QemuDeviceI6300esb{})
	// gob.Register(QemuDeviceIb700{})
	// gob.Register(QemuDeviceIntelIommu{})
	// gob.Register(QemuDeviceIsaApplesmc{})
	// gob.Register(QemuDeviceIsaDebugExit{})
	// gob.Register(QemuDeviceIsaDebugcon{})
	// gob.Register(QemuDeviceIvshmemDoorbell{})
	// gob.Register(QemuDeviceIvshmemPlain{})
	// gob.Register(QemuDeviceKvaserPci{})
	// gob.Register(QemuDeviceLoader{})
	// gob.Register(QemuDeviceMioe3680Pci{})
	// gob.Register(QemuDevicePcTestdev{})
	// gob.Register(QemuDevicePciTestdev{})
	// gob.Register(QemuDevicePcm3680Pci{})
	// gob.Register(QemuDevicePvpanic{})
	// gob.Register(QemuDeviceSmbusIpmi{})
	// gob.Register(QemuDeviceTpmCrb{})
	// gob.Register(QemuDeviceUsbRedir{})
	// gob.Register(QemuDeviceVfioPci{})
	// gob.Register(QemuDeviceVfioPciNohotplug{})
	// gob.Register(QemuDeviceVhostUserVsockDevice{})
	// gob.Register(QemuDeviceVhostUserVsockPci{})
	// gob.Register(QemuDeviceVhostUserVsockPciNonTransitional{})
	// gob.Register(QemuDeviceVhostVsockDevice{})
	// gob.Register(QemuDeviceVhostVsockPci{})
	// gob.Register(QemuDeviceVhostVsockPciNonTransitional{})
	// gob.Register(QemuDeviceVirtioBalloonDevice{})
	// gob.Register(QemuDeviceVirtioBalloonPci{})
	// gob.Register(QemuDeviceVirtioBalloonPciNonTransitional{})
	// gob.Register(QemuDeviceVirtioBalloonPciTransitional{})
	// gob.Register(QemuDeviceVirtioCryptoDevice{})
	// gob.Register(QemuDeviceVirtioCryptoPci{})
	// gob.Register(QemuDeviceVirtioIommuDevice{})
	// gob.Register(QemuDeviceVirtioIommuPci{})
	// gob.Register(QemuDeviceVirtioIommuPciNonTransitional{})
	// gob.Register(QemuDeviceVirtioMem{})
	// gob.Register(QemuDeviceVirtioMemPci{})
	// gob.Register(QemuDeviceVirtioPmemPci{})
	// gob.Register(QemuDeviceVirtioRngDevice{})
	// gob.Register(QemuDeviceVirtioRngPci{})
	// gob.Register(QemuDeviceVirtioRngPciNonTransitional{})
	// gob.Register(QemuDeviceVirtioRngPciTransitional{})
	// gob.Register(QemuDeviceVmcoreinfo{})
	// gob.Register(QemuDeviceVmgenid{})
	// gob.Register(QemuDeviceXenBackend{})
	// gob.Register(QemuDeviceXenPciPassthrough{})
	// gob.Register(QemuDeviceXenPlatform{})

	// Network devices
	// gob.Register(QemuDeviceE1000{})
	// gob.Register(QemuDeviceE100082544gc{})
	// gob.Register(QemuDeviceE100082545em{})
	// gob.Register(QemuDeviceE1000e{})
	// gob.Register(QemuDeviceI82550{})
	// gob.Register(QemuDeviceI82551{})
	// gob.Register(QemuDeviceI82557a{})
	// gob.Register(QemuDeviceI82557b{})
	// gob.Register(QemuDeviceI82557c{})
	// gob.Register(QemuDeviceI82558a{})
	// gob.Register(QemuDeviceI82558b{})
	// gob.Register(QemuDeviceI82559a{})
	// gob.Register(QemuDeviceI82559b{})
	// gob.Register(QemuDeviceI82559c{})
	// gob.Register(QemuDeviceI82559er{})
	// gob.Register(QemuDeviceI82562{})
	// gob.Register(QemuDeviceI82801{})
	// gob.Register(QemuDeviceNe2kIsa{})
	// gob.Register(QemuDeviceNe2kPci{})
	// gob.Register(QemuDevicePcnet{})
	// gob.Register(QemuDevicePvrdma{})
	// gob.Register(QemuDeviceRocker{})
	// gob.Register(QemuDeviceRtl8139{})
	// gob.Register(QemuDeviceTulip{})
	// gob.Register(QemuDeviceUsbNet{})
	// gob.Register(QemuDeviceVirtioNetDevice{})
	// gob.Register(QemuDeviceVirtioNetPci{})
	// gob.Register(QemuDeviceVirtioNetPciNonTransitional{})
	// gob.Register(QemuDeviceVirtioNetPciTransitional{})
	// gob.Register(QemuDeviceVmxnet3{})

	// Sound devices
	// gob.Register(QemuDeviceAc97{})
	// gob.Register(QemuDeviceAdlib{})
	// gob.Register(QemuDeviceCs4231a{})
	// gob.Register(QemuDeviceEs1370{})
	// gob.Register(QemuDeviceGus{})
	// gob.Register(QemuDeviceHdaDuplex{})
	// gob.Register(QemuDeviceHdaMicro{})
	// gob.Register(QemuDeviceHdaOutput{})
	// gob.Register(QemuDeviceIch9IntelHda{})
	// gob.Register(QemuDeviceIntelHda{})
	// gob.Register(QemuDeviceSb16{})
	// gob.Register(QemuDeviceUsbAudio{})

	// Storage devices
	// gob.Register(QemuDeviceAm53c974{})
	// gob.Register(QemuDeviceDc390{})
	// gob.Register(QemuDeviceFloppy{})
	// gob.Register(QemuDeviceIch9Ahci{})
	// gob.Register(QemuDeviceIdeCd{})
	// gob.Register(QemuDeviceIdeDrive{})
	// gob.Register(QemuDeviceIdeHd{})
	// gob.Register(QemuDeviceIsaFdc{})
	// gob.Register(QemuDeviceIsaIde{})
	// gob.Register(QemuDeviceLsi53c810{})
	// gob.Register(QemuDeviceLsi53c895a{})
	// gob.Register(QemuDeviceMegasas{})
	// gob.Register(QemuDeviceMegasasGen2{})
	// gob.Register(QemuDeviceMptsas1068{})
	// gob.Register(QemuDeviceNvme{})
	// gob.Register(QemuDeviceNvmeNs{})
	// gob.Register(QemuDevicePiix3Ide{})
	// gob.Register(QemuDevicePiix3IdeXen{})
	// gob.Register(QemuDevicePiix4Ide{})
	// gob.Register(QemuDevicePvscsi{})
	// gob.Register(QemuDeviceScsiBlock{})
	// gob.Register(QemuDeviceScsiCd{})
	// gob.Register(QemuDeviceScsiDisk{})
	// gob.Register(QemuDeviceScsiGeneric{})
	// gob.Register(QemuDeviceScsiHd{})
	// gob.Register(QemuDeviceSdCard{})
	// gob.Register(QemuDeviceSdhciPci{})
	// gob.Register(QemuDeviceUsbBot{})
	// gob.Register(QemuDeviceUsbMtp{})
	// gob.Register(QemuDeviceUsbStorage{})
	// gob.Register(QemuDeviceUsbUas{})
	// gob.Register(QemuDeviceVhostScsi{})
	// gob.Register(QemuDeviceVhostScsiPci{})
	// gob.Register(QemuDeviceVhostScsiPciNonTransitional{})
	// gob.Register(QemuDeviceVhostScsiPciTransitional{})
	// gob.Register(QemuDeviceVhostUserBlk{})
	// gob.Register(QemuDeviceVhostUserBlkPci{})
	// gob.Register(QemuDeviceVhostUserBlkPciNonTransitional{})
	// gob.Register(QemuDeviceVhostUserBlkPciTransitional{})
	// gob.Register(QemuDeviceVhostUserFsDevice{})
	// gob.Register(QemuDeviceVhostUserFsPci{})
	// gob.Register(QemuDeviceVhostUserScsi{})
	// gob.Register(QemuDeviceVhostUserScsiPci{})
	// gob.Register(QemuDeviceVhostUserScsiPciNonTransitional{})
	// gob.Register(QemuDeviceVhostUserScsiPciTransitional{})
	// gob.Register(QemuDeviceVirtio9pDevice{})
	// gob.Register(QemuDeviceVirtio9pPci{})
	// gob.Register(QemuDeviceVirtio9pPciNonTransitional{})
	// gob.Register(QemuDeviceVirtio9pPciTransitional{})
	// gob.Register(QemuDeviceVirtioBlkDevice{})
	// gob.Register(QemuDeviceVirtioBlkPci{})
	// gob.Register(QemuDeviceVirtioBlkPciNonTransitional{})
	// gob.Register(QemuDeviceVirtioBlkPciTransitional{})
	// gob.Register(QemuDeviceVirtioScsiDevice{})
	// gob.Register(QemuDeviceVirtioScsiPci{})
	// gob.Register(QemuDeviceVirtioScsiPciNonTransitional{})
	// gob.Register(QemuDeviceVirtioScsiPciTransitional{})

	// USB devices
	// gob.Register(QemuDeviceIch9UsbEhci1{})
	// gob.Register(QemuDeviceIch9UsbEhci2{})
	// gob.Register(QemuDeviceIch9UsbUhci1{})
	// gob.Register(QemuDeviceIch9UsbUhci2{})
	// gob.Register(QemuDeviceIch9UsbUhci3{})
	// gob.Register(QemuDeviceIch9UsbUhci4{})
	// gob.Register(QemuDeviceIch9UsbUhci5{})
	// gob.Register(QemuDeviceIch9UsbUhci6{})
	// gob.Register(QemuDeviceNecUsbXhci{})
	// gob.Register(QemuDevicePciOhci{})
	// gob.Register(QemuDevicePiix3UsbUhci{})
	// gob.Register(QemuDevicePiix4UsbUhci{})
	// gob.Register(QemuDeviceQemuXhci{})
	// gob.Register(QemuDeviceUsbEhci{})
	// gob.Register(QemuDeviceVt82c686bUsbUhci{})

	// Uncategorized devices
	// gob.Register(QemuDeviceAmdviPci{})
	// gob.Register(QemuDeviceIpmiBmcExtern{})
	// gob.Register(QemuDeviceIpmiBmcSim{})
	// gob.Register(QemuDeviceIsaIpmiBt{})
	// gob.Register(QemuDeviceIsaIpmiKcs{})
	// gob.Register(QemuDeviceMc146818rtc{})
	// gob.Register(QemuDeviceNvdimm{})
	// gob.Register(QemuDevicePcDimm{})
	// gob.Register(QemuDevicePciIpmiBt{})
	// gob.Register(QemuDevicePciIpmiKcs{})
	// gob.Register(QemuDeviceTpmTis{})
	// gob.Register(QemuDeviceU2fPassthru{})
	// gob.Register(QemuDeviceVirtioPmem{})
	// gob.Register(QemuDeviceVmmouse{})
	// gob.Register(QemuDeviceXenCdrom{})
	// gob.Register(QemuDeviceXenDisk{})
	// gob.Register(QemuDeviceXenPvdevice{})

	// CPUs
	gob.Register(QemuCPU{})
	gob.Register(QemuCPUX86(""))
	gob.Register(QemuCPUArm(""))

	// Displays
	// gob.Register(QemuDisplaySpiceApp{})
	// gob.Register(QemuDisplayGtk{})
	// gob.Register(QemuDisplayVNC{})
	// gob.Register(QemuDisplayCurses{})
	// gob.Register(QemuDisplayEglHeadless{})
	gob.Register(QemuDisplayNone{})
}

func NewQemuDriver(opts ...driveropts.DriverOption) (*QemuDriver, error) {
	dopts, err := driveropts.NewDriverOptions(opts...)
	if err != nil {
		return nil, err
	}

	if dopts.Store == nil {
		return nil, fmt.Errorf("cannot instantiate QEMU driver without machine store")
	}

	driver := QemuDriver{
		dopts: dopts,
	}

	return &driver, nil
}

func (qd *QemuDriver) Create(ctx context.Context, opts ...machine.MachineOption) (machine.MachineID, error) {
	mcfg, err := machine.NewMachineConfig(opts...)
	if err != nil {
		return machine.NullMachineID, fmt.Errorf("could build machine config: %v", err)
	}

	mid, err := machine.NewRandomMachineID()
	if err != nil {
		return machine.NullMachineID, fmt.Errorf("could not generate new machine ID: %v", err)
	}

	mcfg.ID = mid

	pidFile := filepath.Join(qd.dopts.RuntimeDir, mid.String()+".pid")
	qopts := []QemuOption{
		WithDaemonize(true),
		WithEnableKVM(true),
		WithNoGraphic(true),
		WithNoReboot(true),
		WithNoStart(true),
		WithPidFile(pidFile),
		WithName(mid.String()),
		WithKernel(mcfg.KernelPath),
		WithAppend(mcfg.Arguments...),
		WithVGA(QemuVGANone),
		WithMemory(QemuMemory{
			Size: mcfg.MemorySize,
			Unit: QemuMemoryUnitMB,
		}),
		// Create a QMP connection solely for manipulating the machine
		WithQMP(QemuHostCharDevUnix{
			SocketDir: qd.dopts.RuntimeDir,
			Name:      mid.String() + "_control",
			NoWait:    true,
			Server:    true,
		}),
		// Create a QMP connection solely for listening to events
		WithQMP(QemuHostCharDevUnix{
			SocketDir: qd.dopts.RuntimeDir,
			Name:      mid.String() + "_events",
			NoWait:    true,
			Server:    true,
		}),
		WithSerial(QemuHostCharDevUnix{
			SocketDir: qd.dopts.RuntimeDir,
			Name:      mid.String() + "_serial",
			NoWait:    true,
			Server:    true,
		}),
		WithMonitor(QemuHostCharDevUnix{
			SocketDir: qd.dopts.RuntimeDir,
			Name:      mid.String() + "_mon",
			NoWait:    true,
			Server:    true,
		}),
		WithSMP(QemuSMP{
			CPUs:    mcfg.NumVCPUs,
			Threads: 1,
			Sockets: 1,
		}),
		WithVGA(QemuVGANone),
		WithRTC(QemuRTC{
			Base: QemuRTCBaseUtc,
		}),
		WithDisplay(QemuDisplayNone{}),
		WithParallel(QemuHostCharDevNone{}),
	}

	if len(mcfg.InitrdPath) > 0 {
		qopts = append(qopts,
			WithInitRd(mcfg.InitrdPath),
		)
	}

	var bin string

	switch mcfg.Architecture {
	case "x86_64", "amd64":
		bin = QemuSystemX86

		if mcfg.HardwareAcceleration {
			qopts = append(qopts,
				WithMachine(QemuMachine{
					Type:         QemuMachineTypePC,
					Accelerators: []QemuMachineAccelerator{QemuMachineAccelKVM},
				}),
				WithCPU(QemuCPU{
					CPU: QemuCPUX86Host,
					On:  QemuCPUFeatures{QemuCPUFeatureX2apic},
					Off: QemuCPUFeatures{QemuCPUFeaturePmu},
				}),
			)
		} else {
			qopts = append(qopts,
				WithMachine(QemuMachine{
					Type: QemuMachineTypePC,
				}),
				WithCPU(QemuCPU{
					CPU: QemuCPUX86Qemu64,
					On:  QemuCPUFeatures{QemuCPUFeatureVmx},
					Off: QemuCPUFeatures{QemuCPUFeatureSvm},
				}),
			)
		}

		qopts = append(qopts,
			WithDevice(QemuDeviceSga{}),
		)

	case "arm":
		bin = QemuSystemArm

		qopts = append(qopts,
			WithMachine(QemuMachine{
				Type: QemuMachineTypeVirt,
			}),
			WithCPU(QemuCPU{
				CPU: QemuCPUArmCortexA53,
			}),
		)

	default:
		return machine.NullMachineID, fmt.Errorf("unsupported architecture: %s", mcfg.Architecture)
	}

	qcfg, err := NewQemuConfig(qopts...)
	if err != nil {
		return machine.NullMachineID, fmt.Errorf("could not generate QEMU config: %v", err)
	}

	e, err := exec.NewExecutable(bin, *qcfg)
	if err != nil {
		return machine.NullMachineID, fmt.Errorf("could not prepare QEMU executable: %v", err)
	}

	process, err := exec.NewProcessFromExecutable(e, qd.dopts.ExecOptions...)
	if err != nil {
		return machine.NullMachineID, fmt.Errorf("could not prepare QEMU process: %v", err)
	}

	mcfg.CreatedAt = time.Now()

	// Start and also wait for the process to quit as we have invoked
	// daemonization of the process.  When it exits, we'll have a PID we can use
	// to manipulate the VMM.
	if err := process.StartAndWait(); err != nil {
		return machine.NullMachineID, fmt.Errorf("could not start and wait for QEMU process: %v", err)
	}

	defer func() {
		if err != nil {
			qd.Destroy(ctx, mid)
		}
	}()

	if err = qd.dopts.Store.SaveMachineConfig(mid, *mcfg); err != nil {
		return machine.NullMachineID, fmt.Errorf("could not save machine config: %v", err)
	}

	if err = qd.dopts.Store.SaveDriverConfig(mid, *qcfg); err != nil {
		return machine.NullMachineID, fmt.Errorf("could not save driver config: %v", err)
	}

	if err = qd.dopts.Store.SaveMachineState(mid, machine.MachineStateCreated); err != nil {
		return machine.NullMachineID, fmt.Errorf("could not save machine state: %v", err)
	}

	return mid, nil
}

func (qd *QemuDriver) Config(ctx context.Context, mid machine.MachineID) (*QemuConfig, error) {
	dcfg := &QemuConfig{}

	if err := qd.dopts.Store.LookupDriverConfig(mid, dcfg); err != nil {
		return nil, err
	}

	return dcfg, nil
}

func qmpClientHandshake(conn *net.Conn) (*qmpv1alpha.QEMUMachineProtocolClient, error) {
	qmpClient := qmpv1alpha.NewQEMUMachineProtocolClient(*conn)

	greeting, err := qmpClient.Greeting()
	if err != nil {
		return nil, err
	}

	_, err = qmpClient.Capabilities(qmpv1alpha.CapabilitiesRequest{
		Arguments: qmpv1alpha.CapabilitiesRequestArguments{
			Enable: greeting.Qmp.Capabilities,
		},
	})
	if err != nil {
		return nil, err
	}

	return qmpClient, nil
}

func (qd *QemuDriver) QMPClient(ctx context.Context, mid machine.MachineID) (*qmpv1alpha.QEMUMachineProtocolClient, error) {
	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return nil, err
	}

	// Always use index 0 for manipulating the machine
	conn, err := qcfg.QMP[0].Connection()
	if err != nil {
		return nil, err
	}

	return qmpClientHandshake(&conn)
}

func (qd *QemuDriver) Pid(ctx context.Context, mid machine.MachineID) (uint32, error) {
	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return 0, err
	}

	pidData, err := os.ReadFile(qcfg.PidFile)
	if err != nil {
		return 0, fmt.Errorf("could not read pid file: %v", err)
	}

	pid, err := strconv.ParseUint(strings.TrimSpace(string(pidData)), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("could not convert pid string \"%s\" to uint64: %v", pidData, err)
	}

	return uint32(pid), nil
}

func processFromPidFile(pidFile string) (*goprocess.Process, error) {
	pidData, err := os.ReadFile(pidFile)
	if err != nil {
		return nil, fmt.Errorf("could not read pid file: %v", err)
	}

	pid, err := strconv.ParseUint(strings.TrimSpace(string(pidData)), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("could not convert pid string \"%s\" to uint64: %v", pidData, err)
	}

	process, err := goprocess.NewProcess(int32(pid))
	if err != nil {
		return nil, fmt.Errorf("could not look up process %d: %v", pid, err)
	}

	return process, nil
}

func (qd *QemuDriver) ListenStatusUpdate(ctx context.Context, mid machine.MachineID) (chan machine.MachineState, chan error, error) {
	events := make(chan machine.MachineState)
	errs := make(chan error)

	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return nil, nil, err
	}

	// Always use index 1 for monitoring events
	conn, err := qcfg.QMP[1].Connection()
	if err != nil {
		return nil, nil, err
	}

	// Perform the handshake
	_, err = qmpClientHandshake(&conn)
	if err != nil {
		return nil, nil, err
	}

	monitor, err := qmp.NewQMPEventMonitor(conn,
		qmpv1alpha.EventTypes(),
		nil,
	)
	if err != nil {
		return nil, nil, err
	}

	// firstCall is used to initialize the channel with the current state of the
	// machine, so that it can be immediately acted upon.
	firstCall := true

	go func() {
	accept:
		for {
			// First check if the context has been cancelled
			select {
			case <-ctx.Done():
				break accept
			default:
			}

			// Check the current state
			state, err := qd.State(ctx, mid)
			if err != nil {
				errs <- err
				continue
			}

			// Initialize with the current state
			if firstCall {
				events <- state
				firstCall = false
			}

			// Listen for changes in state
			event, err := monitor.Accept()
			if err != nil {
				errs <- err
				continue
			}

			// Send the event through the channel
			switch event.Event {
			case qmpv1alpha.EVENT_STOP, qmpv1alpha.EVENT_SUSPEND, qmpv1alpha.EVENT_POWERDOWN:
				events <- machine.MachineStatePaused

			case qmpv1alpha.EVENT_RESUME:
				events <- machine.MachineStateRunning

			case qmpv1alpha.EVENT_RESET, qmpv1alpha.EVENT_WAKEUP:
				events <- machine.MachineStateRestarting

			case qmpv1alpha.EVENT_SHUTDOWN:
				events <- machine.MachineStateExited

				if !qcfg.NoShutdown {
					break accept
				}
			default:
				errs <- fmt.Errorf("unsupported event: %s", event.Event)
			}
		}
	}()

	return events, errs, nil
}

func (qd *QemuDriver) AddBridge() {}

func (qd *QemuDriver) Start(ctx context.Context, mid machine.MachineID) error {
	qmpClient, err := qd.QMPClient(ctx, mid)
	if err != nil {
		return fmt.Errorf("could not start qemu instance: %v", err)
	}

	defer qmpClient.Close()
	_, err = qmpClient.Cont(qmpv1alpha.ContRequest{})
	if err != nil {
		return err
	}

	// TODO: Timeout? Unikernels boot quickly, but a user environment may be
	// saturated...

	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return err
	}

	// Check if the process is alive
	process, err := processFromPidFile(qcfg.PidFile)
	if err != nil {
		return err
	}

	isRunning, err := process.IsRunning()
	if err != nil {
		return err
	}

	if isRunning {
		if err := qd.dopts.Store.SaveMachineState(mid, machine.MachineStateRunning); err != nil {
			return err
		}
	}

	return err
}

func (qd *QemuDriver) exitStatusAndAtFromConfig(ctx context.Context, mid machine.MachineID) (exitStatus int, exitedAt time.Time, err error) {
	exitStatus = -1 // return -1 if the process hasn't started
	exitedAt = time.Time{}

	var mcfg machine.MachineConfig
	if err := qd.dopts.Store.LookupMachineConfig(mid, &mcfg); err != nil {
		return exitStatus, exitedAt, fmt.Errorf("could not look up machine config: %v", err)
	}

	exitStatus = mcfg.ExitStatus
	exitedAt = mcfg.ExitedAt

	return
}

func (qd *QemuDriver) Wait(ctx context.Context, mid machine.MachineID) (exitStatus int, exitedAt time.Time, err error) {
	exitStatus, exitedAt, err = qd.exitStatusAndAtFromConfig(ctx, mid)
	if err != nil {
		return
	}

	events, errs, err := qd.ListenStatusUpdate(ctx, mid)
	if err != nil {
		return
	}

	for {
		select {
		case state := <-events:
			exitStatus, exitedAt, err = qd.exitStatusAndAtFromConfig(ctx, mid)

			switch state {
			case machine.MachineStateExited, machine.MachineStateDead:
				return
			}

		case err2 := <-errs:
			exitStatus, exitedAt, err = qd.exitStatusAndAtFromConfig(ctx, mid)

			if errors.Is(err2, qmp.ErrAcceptedNonEvent) {
				continue
			}

			return

		case <-ctx.Done():
			exitStatus, exitedAt, err = qd.exitStatusAndAtFromConfig(ctx, mid)

			// TODO: Should we return an error if the context is cancelled?
			return
		}
	}
}

func (qd *QemuDriver) StartAndWait(ctx context.Context, mid machine.MachineID) (int, time.Time, error) {
	if err := qd.Start(ctx, mid); err != nil {
		// return -1 if the process hasn't started.
		return -1, time.Time{}, err
	}

	return qd.Wait(ctx, mid)
}

func (qd *QemuDriver) Pause(ctx context.Context, mid machine.MachineID) error {
	qmpClient, err := qd.QMPClient(ctx, mid)
	if err != nil {
		return fmt.Errorf("could not start qemu instance: %v", err)
	}

	defer qmpClient.Close()

	_, err = qmpClient.Stop(qmpv1alpha.StopRequest{})
	if err != nil {
		return err
	}

	if err := qd.dopts.Store.SaveMachineState(mid, machine.MachineStatePaused); err != nil {
		return err
	}

	return nil
}

func (qd *QemuDriver) TailWriter(ctx context.Context, mid machine.MachineID, writer io.Writer) error {
	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return err
	}

	if qcfg.Serial == nil {
		return fmt.Errorf("serial console not available for %s", mid)
	}

	conn, err := qcfg.Serial[0].Connection()
	if err != nil {
		return fmt.Errorf("could not connect to serial for %s: %v", mid, err)
	}

	defer conn.Close()

	buf := make([]byte, 1024)

read:
	for {
		// First check if the context has been cancelled
		select {
		case <-ctx.Done():
			break read
		default:
		}

		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("error reading from serial for %s: %v", mid, err)
			}
		}
		if _, err := writer.Write(buf[:n]); err != nil {
			return fmt.Errorf("error writing output from serial for %s: %v", mid, err)
		}
	}

	return nil
}

func (qd *QemuDriver) State(ctx context.Context, mid machine.MachineID) (state machine.MachineState, err error) {
	state = machine.MachineStateUnknown

	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return
	}

	state, err = qd.dopts.Store.LookupMachineState(mid)
	if err != nil {
		return
	}

	savedState := state

	var mcfg machine.MachineConfig
	if err := qd.dopts.Store.LookupMachineConfig(mid, &mcfg); err != nil {
		return state, fmt.Errorf("could not look up machine config: %v", err)
	}

	// Check if the process is alive, which ultimately indicates to us whether we
	// able to speak to the exposed QMP socket
	process, err := processFromPidFile(qcfg.PidFile)
	activeProcess := false
	if err == nil {
		activeProcess, err = process.IsRunning()
		if err != nil {
			state = machine.MachineStateDead
			activeProcess = false
		}
	}

	exitedAt := mcfg.ExitedAt
	exitStatus := mcfg.ExitStatus

	defer func() {
		if exitStatus >= 0 && mcfg.ExitedAt.IsZero() {
			exitedAt = time.Now()
		}

		// Update the machine config with the latest values if they are different from
		// what we have on record
		if mcfg.ExitedAt != exitedAt || mcfg.ExitStatus != exitStatus {
			mcfg.ExitedAt = exitedAt
			mcfg.ExitStatus = exitStatus
			if err = qd.dopts.Store.SaveMachineConfig(mid, mcfg); err != nil {
				return
			}
		}

		// Finally, save the state if it is different from the what we have on record
		if state != savedState {
			if err = qd.dopts.Store.SaveMachineState(mid, state); err != nil {
				return
			}
		}
	}()

	if !activeProcess {
		return
	}

	qmpClient, err := qd.QMPClient(ctx, mid)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		state = machine.MachineStateDead
		exitStatus = 1
		return
	} else if err != nil {
		return state, fmt.Errorf("could not attach to QMP client: %v", err)
	}

	defer qmpClient.Close()

	// Grab the actual state of the machine by querying QMP
	status, err := qmpClient.QueryStatus(qmpv1alpha.QueryStatusRequest{})
	if err != nil {
		// We cannot amend the status at this point, even if the process is
		// alive, since it is not an indicator of the state of the VM, only of the
		// VMM.  So we return what we already know via LookupMachineConfig.
		return state, fmt.Errorf("could not query machine status via QMP: %v", err)
	}

	// Map the QMP status to supported machine states
	switch status.Return.Status {
	case qmpv1alpha.RUN_STATE_GUEST_PANICKED, qmpv1alpha.RUN_STATE_INTERNAL_ERROR, qmpv1alpha.RUN_STATE_IO_ERROR:
		state = machine.MachineStateDead
		exitStatus = 1

	case qmpv1alpha.RUN_STATE_PAUSED:
		state = machine.MachineStatePaused
		exitStatus = -1

	case qmpv1alpha.RUN_STATE_RUNNING:
		state = machine.MachineStateRunning
		exitStatus = -1

	case qmpv1alpha.RUN_STATE_SHUTDOWN:
		state = machine.MachineStateExited
		exitStatus = 0

	case qmpv1alpha.RUN_STATE_SUSPENDED:
		state = machine.MachineStateSuspended
		exitStatus = -1

	default:
		// qmpv1alpha.RUN_STATE_SAVE_VM,
		// qmpv1alpha.RUN_STATE_PRELAUNCH,
		// qmpv1alpha.RUN_STATE_RESTORE_VM,
		// qmpv1alpha.RUN_STATE_WATCHDOG,
		state = machine.MachineStateUnknown
		exitStatus = -1
	}

	return
}

func (qd *QemuDriver) List(ctx context.Context) ([]machine.MachineID, error) {
	var mids []machine.MachineID

	midmap, err := qd.dopts.Store.ListAllMachineConfigs()
	if err != nil {
		return nil, err
	}

	for mid, mcfg := range midmap {
		if mcfg.DriverName == "qemu" {
			mids = append(mids, mid)
		}
	}

	return mids, nil
}

func (qd *QemuDriver) Stop(ctx context.Context, mid machine.MachineID) error {
	qmpClient, err := qd.QMPClient(ctx, mid)
	if err != nil {
		return err
	}

	defer qmpClient.Close()
	_, err = qmpClient.Quit(qmpv1alpha.QuitRequest{})
	if err != nil {
		return err
	}

	qcfg, err := qd.Config(ctx, mid)
	if err != nil {
		return err
	}

	if err := retrytimeout.RetryTimeout(5*time.Second, func() error {
		if _, err := os.ReadFile(qcfg.PidFile); !os.IsNotExist(err) {
			return fmt.Errorf("process still active")
		}

		return nil
	}); err != nil {
		return err
	}

	if err := qd.dopts.Store.SaveMachineState(mid, machine.MachineStateExited); err != nil {
		return err
	}

	return nil
}

func (qd *QemuDriver) Destroy(ctx context.Context, mid machine.MachineID) error {
	state, err := qd.dopts.Store.LookupMachineState(mid)
	if err != nil {
		return err
	}

	switch state {
	case machine.MachineStateUnknown,
		machine.MachineStateExited,
		machine.MachineStateDead:
	default:
		qd.Stop(ctx, mid)
	}

	return qd.dopts.Store.Purge(mid)
}

func (qd *QemuDriver) Shutdown(ctx context.Context, mid machine.MachineID) error {
	qmpClient, err := qd.QMPClient(ctx, mid)
	if err != nil {
		return err
	}

	defer qmpClient.Close()
	_, err = qmpClient.SystemPowerdown(qmpv1alpha.SystemPowerdownRequest{})
	if err != nil {
		return err
	}

	return nil
}

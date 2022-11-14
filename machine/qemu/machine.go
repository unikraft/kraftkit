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

import "strings"

type QemuMachineType string

const (
	QemuMachineTypeVirt    = QemuMachineType("virt")
	QemuMachineTypePC      = QemuMachineType("pc")
	QemuMachineTypeMicroVM = QemuMachineType("microvm")
	QemuMachineTypeQ35     = QemuMachineType("q35")
	QemuMachineTypeNone    = QemuMachineType("none")
	QemuMachineTypeXenPV   = QemuMachineType("xenpv")
	QemuMachineTypeXenFV   = QemuMachineType("xenfv")
	QemuMachineTypeISAPC   = QemuMachineType("isapc")
)

func (qmt QemuMachineType) String() string {
	return string(qmt)
}

type QemuMachineAccelerator string

const (
	QemuMachineAccelKVM  = QemuMachineAccelerator("kvm")
	QemuMachineAccelXen  = QemuMachineAccelerator("xen")
	QemuMachineAccelHVF  = QemuMachineAccelerator("hvf")
	QemuMachineAccelWHPX = QemuMachineAccelerator("whpx")
	QemuMachineAccelTCG  = QemuMachineAccelerator("tcg")
)

func (qma QemuMachineAccelerator) String() string {
	return string(qma)
}

type QemuMachineOptOnOffAuto string

const (
	QemuMachineOptOn   = QemuMachineOptOnOffAuto("on")
	QemuMachineOptOff  = QemuMachineOptOnOffAuto("off")
	QemuMachineOptAuto = QemuMachineOptOnOffAuto("auto")
)

type QemuMachine struct {
	Type          QemuMachineType          `json_name:"type,omitempty"`
	Accelerators  []QemuMachineAccelerator `json_name:"accelerator,omitempty"`
	VMPort        QemuMachineOptOnOffAuto  `json_name:"vmport,omitempty"`
	DumpGuestCore bool                     `json_name:"dump-guest-core,omitempty"`
	MemMerge      bool                     `json_name:"mem-merge,omitempty"`
	AESKeyWrap    bool                     `json_name:"qes-key-wrap,omitempty"`
	DEAKeyWrap    bool                     `json_name:"dea-key-wrap,omitempty"`
	SupressVMDesc bool                     `json_name:"suppress-vmdesc,omitempty"`
	NVDIMM        bool                     `json_name:"nvdimm,omitempty"`
	HMAT          bool                     `json_name:"hmat,omitempty"`
}

// String returns a QEMU command-line compatible -machine flag value
func (qm QemuMachine) String() string {
	if len(qm.Type) == 0 {
		// Cannot return machine configuration with unset type
		return ""
	}

	var ret strings.Builder

	ret.WriteString(string(qm.Type))

	if len(qm.Accelerators) > 0 {
		ret.WriteString(",accel=")

		var (
			sep = []byte(":")
			// preallocate for len(sep) + assume at least 1 character
			out = make([]byte, 0, (1+len(sep))*len(qm.Accelerators))
		)
		for _, s := range qm.Accelerators {
			out = append(out, s...)
			out = append(out, sep...)
		}

		ret.WriteString(string(out[:len(out)-len(sep)]))
	}

	if string(qm.VMPort) != "" {
		ret.WriteString(",vmport=")
		ret.WriteString(string(qm.VMPort))
	}
	if qm.DumpGuestCore {
		ret.WriteString(",dump-guest-core=on")
	}
	if qm.MemMerge {
		ret.WriteString(",mem-merge=on")
	}
	if qm.AESKeyWrap {
		ret.WriteString(",aes-key-wrap=on")
	}
	if qm.DEAKeyWrap {
		ret.WriteString(",dea-key-wrap=on")
	}
	if qm.SupressVMDesc {
		ret.WriteString(",suppress-vmdesc=on")
	}
	if qm.NVDIMM {
		ret.WriteString(",nvdimm=on")
	}
	if qm.HMAT {
		ret.WriteString(",hmat=on")
	}

	return ret.String()
}

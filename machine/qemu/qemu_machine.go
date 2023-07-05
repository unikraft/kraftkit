// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
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
	Type          QemuMachineType          `json:"type,omitempty"`
	Accelerators  []QemuMachineAccelerator `json:"accelerator,omitempty"`
	VMPort        QemuMachineOptOnOffAuto  `json:"vmport,omitempty"`
	DumpGuestCore bool                     `json:"dump_guest_core,omitempty"`
	MemMerge      bool                     `json:"mem_merge,omitempty"`
	AESKeyWrap    bool                     `json:"qes_key_wrap,omitempty"`
	DEAKeyWrap    bool                     `json:"dea_key_wrap,omitempty"`
	SupressVMDesc bool                     `json:"suppress_vmdesc,omitempty"`
	NVDIMM        bool                     `json:"nvdimm,omitempty"`
	HMAT          bool                     `json:"hmat,omitempty"`

	// Added in QEMU 8.0.0
	Graphics bool `json:"graphics,omitempty"`
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

	// Added in QEMU 8.0.0
	if qm.HMAT {
		ret.WriteString(",graphics=on")
	}

	return ret.String()
}

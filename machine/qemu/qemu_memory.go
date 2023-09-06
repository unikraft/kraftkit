// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

import (
	"strconv"
	"strings"
)

type QemuMemoryUnit string

const (
	QemuMemoryUnitMB = QemuMemoryUnit("M")
	QemuMemoryUnitGB = QemuMemoryUnit("G")
)

type QemuMemory struct {
	Size   uint64         `json:"size,omitempty"`
	Unit   QemuMemoryUnit `json:"unit,omitempty"`
	Slots  uint64         `json:"slots,omitempty"`
	MaxMem string         `json:"max_mem,omitempty"`
}

const (
	QemuMemoryDefault = 64
	QemuMemoryScale   = 1024 * 1024
)

func (qm QemuMemory) String() string {
	if qm.Size == 0 && len(qm.Unit) == 0 {
		return ""
	}

	var ret strings.Builder

	if qm.Size == 0 {
		qm.Size = QemuMemoryDefault
	}
	if len(qm.Unit) == 0 {
		qm.Unit = QemuMemoryUnitMB
	}

	ret.WriteString("size=")
	ret.WriteString(strconv.FormatUint(qm.Size, 10))
	ret.WriteString(string(qm.Unit))

	if qm.Slots > 0 {
		ret.WriteString("slots=")
		ret.WriteString(strconv.FormatUint(qm.Slots, 10))
	}

	if len(qm.MaxMem) > 0 {
		ret.WriteString("maxmem=")
		ret.WriteString(qm.MaxMem)
	}

	return ret.String()
}

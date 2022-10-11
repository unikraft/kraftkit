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
)

func (qm QemuMemory) String() string {
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

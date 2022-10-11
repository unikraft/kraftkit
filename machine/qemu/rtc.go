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
	"strings"
	"time"
)

type QemuRTCBaseType string

const (
	QemuRTCBaseUtc       = QemuRTCBaseType("utc")
	QemuRTCBaseLocaltime = QemuRTCBaseType("localtime")
	QemuRTCBaseCustom    = QemuRTCBaseType("custom")
)

type QemuRTCClockType string

const (
	QemuRTCClockHost = QemuRTCClockType("host")
	QemuRTCClockRt   = QemuRTCClockType("rt")
	QemuRTCClockVm   = QemuRTCClockType("vm")
)

type QemuRTCDriftFixType string

const (
	QemuRTCDriftFixNone = QemuRTCDriftFixType("none")
	QemuRTCDriftFixSlew = QemuRTCDriftFixType("slew")
)

type QemuRTC struct {
	Base         QemuRTCBaseType     `json:"base,omitempty"`
	BaseDatetime time.Time           `json:"base_datetime,omitempty"`
	Clock        QemuRTCClockType    `json:"clock,omitempty"`
	DriftFix     QemuRTCDriftFixType `json:"drift_fix,omitempty"`
}

func (qr QemuRTC) String() string {
	var ret strings.Builder
	ret.WriteString("base=")

	if qr.Base == QemuRTCBaseCustom {
		ret.WriteString(qr.BaseDatetime.String())
	} else if qr.Base != "" {
		ret.WriteString(string(qr.Base))
	}
	if qr.Clock != "" {
		ret.WriteString(",clock=")
		ret.WriteString(string(qr.Clock))
	}
	if qr.DriftFix != "" {
		ret.WriteString(",driftfix=")
		ret.WriteString(string(qr.DriftFix))
	}

	return ret.String()
}

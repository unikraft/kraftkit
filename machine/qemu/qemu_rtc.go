// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
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
	if qr.Base == "" {
		return ""
	}

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

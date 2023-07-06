// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

import (
	"strconv"
	"strings"
)

type QemuSMP struct {
	// CPUs sets the number of CPUs (default is 1)
	CPUs uint64 `json:"cpus,omitempty"`

	// MaxCPUs is the maximum number of total CPUs, including offline CPUs for
	// hotplug, etc.
	MaxCPUs uint64 `json:"max_cpus,omitempty"`

	// Cores is the number of CPU cores on one socket (for PC, it's on one die)
	Cores uint64 `json:"cores,omitempty"`

	// Threads is the number of threads on one CPU core
	Threads uint64 `json:"threads,omitempty"`

	// Dies is the number of CPU dies on one socket (for PC only)
	Dies uint64 `json:"dies,omitempty"`

	// Sockets is the number of discrete sockets in the system
	Sockets uint64 `json:"sockets,omitempty"`
}

// String returns a QEMU command-line compatible -smp flag value in the format:
// [cpus=]n[,maxcpus=cpus][,cores=cores][,threads=threads][,dies=dies]
// [,sockets=sockets]
func (qsmp QemuSMP) String() string {
	if qsmp.CPUs <= 0 {
		return ""
	}

	var ret strings.Builder

	ret.WriteString("cpus=")
	ret.WriteString(strconv.FormatUint(qsmp.CPUs, 10))

	if qsmp.MaxCPUs > 0 {
		ret.WriteString(",maxcpus=")
		ret.WriteString(strconv.FormatUint(qsmp.MaxCPUs, 10))
	}
	if qsmp.Cores > 0 {
		ret.WriteString(",cores=")
		ret.WriteString(strconv.FormatUint(qsmp.Cores, 10))
	}
	if qsmp.Threads > 0 {
		ret.WriteString(",threads=")
		ret.WriteString(strconv.FormatUint(qsmp.Threads, 10))
	}
	if qsmp.Dies > 0 {
		ret.WriteString(",dies=")
		ret.WriteString(strconv.FormatUint(qsmp.Dies, 10))
	}
	if qsmp.Sockets > 0 {
		ret.WriteString(",sockets=")
		ret.WriteString(strconv.FormatUint(qsmp.Sockets, 10))
	}

	return ret.String()
}

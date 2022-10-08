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
	var ret strings.Builder

	if qsmp.CPUs <= 0 {
		qsmp.CPUs = 1
	}

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

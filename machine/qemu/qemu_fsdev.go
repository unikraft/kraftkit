// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

import (
	"fmt"
	"strconv"
	"strings"
)

type QemuFsDev interface {
	fmt.Stringer
}

type QemuFsDevType string

const (
	QemuFsDevTypeLocal = QemuFsDevType("local")
	QemuFsDevTypeProxy = QemuFsDevType("proxy")
	QemuFsDevTypeSynth = QemuFsDevType("synth")
)

type QemuFsDevLocalSecurityModel string

const (
	QemuFsDevLocalSecurityModelMappedFile  = QemuFsDevLocalSecurityModel("mapped-file")
	QemuFsDevLocalSecurityModelMappedXattr = QemuFsDevLocalSecurityModel("mapped-xattr")
	QemuFsDevLocalSecurityModelNone        = QemuFsDevLocalSecurityModel("none")
	QemuFsDevLocalSecurityModelPassthrough = QemuFsDevLocalSecurityModel("passthrough")
)

type QemuFsDevLocal struct {
	Id            string                      `json:"id,omitempty"`
	Path          string                      `json:"path,omitempty"`
	SecurityModel QemuFsDevLocalSecurityModel `json:"security_model,omitempty"`
	Writeout      string                      `json:"writeout,omitempty"`
	Readonly      bool                        `json:"readonly,omitempty"`
	Fmode         string                      `json:"fmode,omitempty"`
	Dmode         string                      `json:"dmode,omitempty"`
	Throttling    struct {
		// length of the bps-read-max burst period, in seconds
		BpsReadMaxLength int `json:"bps-read-max-length,omitempty"`
		// total bytes read burst
		BpsReadMax int `json:"bps-read-max,omitempty"`
		// limit read bytes per second
		BpsRead int `json:"bps-read,omitempty"`
		// length of the bps-total-max burst period, in seconds
		BpsTotalMaxLength int `json:"bps-total-max-length,omitempty"`
		// total bytes burst
		BpsTotalMax int `json:"bps-total-max,omitempty"`
		// limit total bytes per second
		BpsTotal int `json:"bps-total,omitempty"`
		// length of the bps-write-max burst period, in seconds
		BpsWriteMaxLength int `json:"bps-write-max-length,omitempty"`
		// total bytes write burst
		BpsWriteMax int `json:"bps-write-max,omitempty"`
		// limit write bytes per second
		BpsWrite int `json:"bps-write,omitempty"`
		// length of the iops-read-max burst period, in seconds
		IopsReadMaxLength int `json:"iops-read-max-length,omitempty"`
		// I/O operations read burst
		IopsReadMax int `json:"iops-read-max,omitempty"`
		// limit read operations per second
		IopsRead int `json:"iops-read,omitempty"`
		// when limiting by iops max size of an I/O in bytes
		IopsSize int `json:"iops-size,omitempty"`
		// length of the iops-total-max burst period, in seconds
		IopsTotalMaxLength int `json:"iops-total-max-length,omitempty"`
		// I/O operations burst
		IopsTotalMax int `json:"iops-total-max,omitempty"`
		// limit total I/O operations per second
		IopsTotal int `json:"iops-total,omitempty"`
		// length of the iops-write-max burst period, in seconds
		IopsWriteMaxLength int `json:"iops-write-max-length,omitempty"`
		// I/O operations write burst
		IopsWriteMax int `json:"iops-write-max,omitempty"`
		// limit write operations per second
		IopsWrite int `json:"iops-write,omitempty"`
	} `json:"throttling,omitempty"`
}

// String returns a QEMU command-line compatible fsdev string with the format:
// local,id=id,path=path,security_model=mapped-xattr|mapped-file|passthrough|none
// [,writeout=immediate][,readonly][,fmode=fmode][,dmode=dmode]
// [[,throttling.bps-total=b]|[[,throttling.bps-read=r][,throttling.bps-write=w]]]
// [[,throttling.iops-total=i]|[[,throttling.iops-read=r][,throttling.iops-write=w]]]
// [[,throttling.bps-total-max=bm]|[[,throttling.bps-read-max=rm][,throttling.bps-write-max=wm]]]
// [[,throttling.iops-total-max=im]|[[,throttling.iops-read-max=irm][,throttling.iops-write-max=iwm]]]
// [[,throttling.iops-size=is]]
func (fd QemuFsDevLocal) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuFsDevTypeLocal))
	ret.WriteString(",id=")
	ret.WriteString(fd.Id)
	ret.WriteString(",path=")
	ret.WriteString(fd.Path)
	ret.WriteString(",security_model=")
	ret.WriteString(string(fd.SecurityModel))

	if len(fd.Writeout) > 0 {
		ret.WriteString(",writeout=")
		ret.WriteString(fd.Writeout)
	}
	if fd.Readonly {
		ret.WriteString(",readonly")
	}
	if len(fd.Fmode) > 0 {
		ret.WriteString(",fmode=")
		ret.WriteString(fd.Fmode)
	}
	if len(fd.Dmode) > 0 {
		ret.WriteString(",dmode=")
		ret.WriteString(fd.Dmode)
	}
	if fd.Throttling.BpsReadMaxLength > 0 {
		ret.WriteString(",throttling.bps-read-max-length=")
		ret.WriteString(strconv.Itoa(fd.Throttling.BpsReadMaxLength))
	}
	if fd.Throttling.BpsReadMax > 0 {
		ret.WriteString(",throttling.bps-read-max=")
		ret.WriteString(strconv.Itoa(fd.Throttling.BpsReadMax))
	}
	if fd.Throttling.BpsRead > 0 {
		ret.WriteString(",throttling.bps-read=")
		ret.WriteString(strconv.Itoa(fd.Throttling.BpsRead))
	}
	if fd.Throttling.BpsTotalMaxLength > 0 {
		ret.WriteString(",throttling.bps-total-max-length=")
		ret.WriteString(strconv.Itoa(fd.Throttling.BpsTotalMaxLength))
	}
	if fd.Throttling.BpsTotalMax > 0 {
		ret.WriteString(",throttling.bps-total-max=")
		ret.WriteString(strconv.Itoa(fd.Throttling.BpsTotalMax))
	}
	if fd.Throttling.BpsTotal > 0 {
		ret.WriteString(",throttling.bps-total=")
		ret.WriteString(strconv.Itoa(fd.Throttling.BpsTotal))
	}
	if fd.Throttling.BpsWriteMaxLength > 0 {
		ret.WriteString(",throttling.bps-write-max-length=")
		ret.WriteString(strconv.Itoa(fd.Throttling.BpsWriteMaxLength))
	}
	if fd.Throttling.BpsWriteMax > 0 {
		ret.WriteString(",throttling.bps-write-max=")
		ret.WriteString(strconv.Itoa(fd.Throttling.BpsWriteMax))
	}
	if fd.Throttling.BpsWrite > 0 {
		ret.WriteString(",throttling.bps-write=")
		ret.WriteString(strconv.Itoa(fd.Throttling.BpsWrite))
	}
	if fd.Throttling.IopsReadMaxLength > 0 {
		ret.WriteString(",throttling.iops-read-max-length=")
		ret.WriteString(strconv.Itoa(fd.Throttling.IopsReadMaxLength))
	}
	if fd.Throttling.IopsReadMax > 0 {
		ret.WriteString(",throttling.iops-read-max=")
		ret.WriteString(strconv.Itoa(fd.Throttling.IopsReadMax))
	}
	if fd.Throttling.IopsRead > 0 {
		ret.WriteString(",throttling.iops-read=")
		ret.WriteString(strconv.Itoa(fd.Throttling.IopsRead))
	}
	if fd.Throttling.IopsSize > 0 {
		ret.WriteString(",throttling.iops-size=")
		ret.WriteString(strconv.Itoa(fd.Throttling.IopsSize))
	}
	if fd.Throttling.IopsTotalMaxLength > 0 {
		ret.WriteString(",throttling.iops-total-max-length=")
		ret.WriteString(strconv.Itoa(fd.Throttling.IopsTotalMaxLength))
	}
	if fd.Throttling.IopsTotalMax > 0 {
		ret.WriteString(",throttling.iops-total-max=")
		ret.WriteString(strconv.Itoa(fd.Throttling.IopsTotalMax))
	}
	if fd.Throttling.IopsTotal > 0 {
		ret.WriteString(",throttling.iops-total=")
		ret.WriteString(strconv.Itoa(fd.Throttling.IopsTotal))
	}
	if fd.Throttling.IopsWriteMaxLength > 0 {
		ret.WriteString(",throttling.iops-write-max-length=")
		ret.WriteString(strconv.Itoa(fd.Throttling.IopsWriteMaxLength))
	}
	if fd.Throttling.IopsWriteMax > 0 {
		ret.WriteString(",throttling.iops-write-max=")
		ret.WriteString(strconv.Itoa(fd.Throttling.IopsWriteMax))
	}
	if fd.Throttling.IopsWrite > 0 {
		ret.WriteString(",throttling.iops-write=")
		ret.WriteString(strconv.Itoa(fd.Throttling.IopsWrite))
	}

	return ret.String()
}

type QemuFsDevProxy struct {
	Id       string `json:"id,omitempty"`
	Socket   string `json:"socket,omitempty"`
	SockFd   int    `json:"sock_fd,omitempty"`
	Writeout string `json:"writeout,omitempty"`
	Readonly bool   `json:"readonly,omitempty"`
}

// String returns a QEMU command-line compatible fsdev string with the format:
// proxy,id=id,socket=socket[,writeout=immediate][,readonly]
// proxy,id=id,sock_fd=sock_fd[,writeout=immediate][,readonly]
func (fd QemuFsDevProxy) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuFsDevTypeSynth))
	ret.WriteString(",id=")
	ret.WriteString(fd.Id)

	if len(fd.Socket) > 0 {
		ret.WriteString(",socket=")
		ret.WriteString(fd.Socket)
	} else if fd.SockFd > 0 {
		ret.WriteString(",sock_fd=")
		ret.WriteString(strconv.Itoa(fd.SockFd))
	}
	if len(fd.Writeout) > 0 {
		ret.WriteString(",writeout=")
		ret.WriteString(fd.Writeout)
	}
	if fd.Readonly {
		ret.WriteString(",readonly")
	}

	return ret.String()
}

type QemuFsDevSynth struct {
	Id string `json:"id,omitempty"`
}

// String returns a QEMU command-line compatible fsdev string with the format:
// synth,id=id
func (fd QemuFsDevSynth) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuFsDevTypeSynth))
	ret.WriteString(",id=")
	ret.WriteString(fd.Id)

	return ret.String()
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package qemu

import (
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
)

type QemuHostCharDev interface {
	fmt.Stringer
	Resource() string
	Connection() (net.Conn, error)
}

type QemuHostCharDevVirtualConsole struct {
	Monitor bool   `json:"monitor,omitempty"`
	Width   string `json:"width,omitempty"`
	Height  string `json:"height,omitempty"`
}

func (cd QemuHostCharDevVirtualConsole) String() string {
	var ret strings.Builder

	if cd.Monitor {
		ret.WriteString("monitor:")
	}

	ret.WriteString("vc")
	ret.WriteString(cd.Resource())

	return ret.String()
}

func (cd QemuHostCharDevVirtualConsole) Resource() string {
	var ret strings.Builder

	if len(cd.Width) > 0 && len(cd.Height) > 0 {
		ret.WriteString(":")
		ret.WriteString(cd.Width)
		ret.WriteString("x")
		ret.WriteString(cd.Height)
	}

	return ret.String()
}

func (cd QemuHostCharDevVirtualConsole) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevVirtualConsole.Connection")
}

type QemuHostCharDevPty struct{}

func (cd QemuHostCharDevPty) String() string {
	return string(QemuCharDevTypePty)
}

func (cd QemuHostCharDevPty) Resource() string {
	return ""
}

func (cd QemuHostCharDevPty) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevPty.Connection")
}

type QemuHostCharDevNone struct{}

func (cd QemuHostCharDevNone) String() string {
	return string(QemuCharDevTypeNone)
}

func (cd QemuHostCharDevNone) Resource() string {
	return ""
}

func (cd QemuHostCharDevNone) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevNone.Connection")
}

type QemuHostCharDevNull struct{}

// String returns a ...
func (cd QemuHostCharDevNull) String() string {
	return string(QemuCharDevTypeNull)
}

func (cd QemuHostCharDevNull) Resource() string {
	return ""
}

func (cd QemuHostCharDevNull) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevNull.Connection")
}

type QemuHostCharDevNamed struct {
	Monitor bool   `json:"monitor,omitempty"`
	Id      string `json:"id,omitempty"`
}

func (cd QemuHostCharDevNamed) String() string {
	if len(cd.Id) == 0 {
		// Cannot stringify named character device without id
		return ""
	}

	var ret strings.Builder

	if cd.Monitor {
		ret.WriteString("monitor:")
	}

	ret.WriteString("chardev:")
	ret.WriteString(cd.Resource())

	return ret.String()
}

func (cd QemuHostCharDevNamed) Resource() string {
	return cd.Id
}

func (cd QemuHostCharDevNamed) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevNamed.Connection")
}

type QemuHostCharDevTty struct {
	Monitor bool   `json:"monitor,omitempty"`
	Path    string `json:"path,omitempty"`
}

func (cd QemuHostCharDevTty) String() string {
	if len(cd.Path) == 0 {
		// Cannot stringify TTY character device without path
		return ""
	} else if !strings.HasPrefix("/dev/", cd.Path) {
		// Invalid TTY device path
		return ""
	}

	var ret strings.Builder

	if cd.Monitor {
		ret.WriteString("monitor:")
	}

	ret.WriteString(cd.Resource())

	return ret.String()
}

func (cd QemuHostCharDevTty) Resource() string {
	return cd.Path
}

func (cd QemuHostCharDevTty) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevTty.Connection")
}

type QemuHostCharDevFile struct {
	Monitor  bool   `json:"monitor,omitempty"`
	Filename string `json:"filename,omitempty"`
}

func (cd QemuHostCharDevFile) String() string {
	if len(cd.Filename) == 0 {
		// Cannot stringify file character device without filename
		return ""
	}

	var ret strings.Builder

	if cd.Monitor {
		ret.WriteString("monitor:")
	}

	ret.WriteString("file:")
	ret.WriteString(cd.Resource())

	return ret.String()
}

func (cd QemuHostCharDevFile) Resource() string {
	return cd.Filename
}

func (cd QemuHostCharDevFile) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevFile.Connection")
}

type QemuHostCharDevStdio struct {
	Monitor bool `json:"monitor,omitempty"`
}

func (cd QemuHostCharDevStdio) String() string {
	var ret strings.Builder

	if cd.Monitor {
		ret.WriteString("monitor:")
	}

	ret.WriteString(cd.Resource())

	return ret.String()
}

func (cd QemuHostCharDevStdio) Resource() string {
	return string(QemuCharDevTypeStdio)
}

func (cd QemuHostCharDevStdio) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevStdio.Connection")
}

type QemuHostCharDevPipe struct {
	Monitor  bool   `json:"monitor,omitempty"`
	Filename string `json:"filename,omitempty"`
}

func (cd QemuHostCharDevPipe) String() string {
	if len(cd.Filename) == 0 {
		// Cannot stringify pipe character device without filename
		return ""
	}

	var ret strings.Builder

	if cd.Monitor {
		ret.WriteString("monitor:")
	}

	ret.WriteString("pipe:")
	ret.WriteString(cd.Resource())

	return ret.String()
}

func (cd QemuHostCharDevPipe) Resource() string {
	return cd.Filename
}

func (cd QemuHostCharDevPipe) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevPipe.Connection")
}

type QemuHostCharDevUDP struct {
	Monitor    bool   `json:"monitor,omitempty"`
	RemoteHost string `json:"remote_host,omitempty"`
	RemotePort int    `json:"remote_port,omitempty"`
	SourceHost string `json:"source_host,omitempty"`
	SourcePort int    `json:"source_port,omitempty"`
}

func (cd QemuHostCharDevUDP) String() string {
	if cd.RemotePort == 0 {
		// Cannot stringify UDP character device without remote port
		return ""
	}

	var ret strings.Builder

	if cd.Monitor {
		ret.WriteString("monitor:")
	}

	ret.WriteString(cd.Resource())

	if len(cd.SourceHost) > 0 || cd.SourcePort > 0 {
		ret.WriteString("@")
		if len(cd.SourceHost) > 0 {
			ret.WriteString(cd.SourceHost)
		}

		ret.WriteString(":")
		ret.WriteString(strconv.Itoa(cd.SourcePort))
	}

	return ret.String()
}

func (cd QemuHostCharDevUDP) Resource() string {
	var ret strings.Builder

	if len(cd.RemoteHost) > 0 {
		ret.WriteString(cd.RemoteHost)
	}

	ret.WriteString(":")
	ret.WriteString(strconv.Itoa(cd.RemotePort))

	return ret.String()
}

func (cd QemuHostCharDevUDP) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevUDP.Connection")
}

const (
	QemuHostCharDevTCPDefaultHost = "0.0.0.0"
	QemuHostCharDevTCPDefaultPort = 4444
)

type QemuHostCharDevTCP struct {
	Monitor   bool   `json:"monitor,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      int    `json:"port,omitempty"`
	Server    bool   `json:"server,omitempty"`
	NoWait    bool   `json:"no_wait,omitempty"`
	NoDelay   bool   `json:"no_delay,omitempty"`
	Reconnect int    `json:"reconnect,omitempty"`
}

func (cd QemuHostCharDevTCP) String() string {
	var ret strings.Builder

	if cd.Monitor {
		ret.WriteString("monitor:")
	}

	ret.WriteString("tcp:")
	ret.WriteString(cd.Resource())

	if cd.Server {
		ret.WriteString(",server")
	}
	if cd.NoWait {
		ret.WriteString(",nowait")
	}
	if cd.NoDelay {
		ret.WriteString(",nodelay")
	}
	if cd.Reconnect > 0 {
		ret.WriteString(",reconnect=")
		ret.WriteString(strconv.Itoa(cd.Reconnect))
	}

	return ret.String()
}

func (cd QemuHostCharDevTCP) Resource() string {
	if cd.Port <= 0 {
		cd.Port = QemuHostCharDevTCPDefaultPort
	}
	if len(cd.Host) <= 0 {
		cd.Host = QemuHostCharDevTCPDefaultHost
	}

	var ret strings.Builder

	ret.WriteString(cd.Host)
	ret.WriteString(":")
	ret.WriteString(strconv.Itoa(cd.Port))

	return ret.String()
}

func (cd QemuHostCharDevTCP) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevTCP.Connection")
}

type QemuHostCharDevTelnet struct {
	Monitor bool   `json:"monitor,omitempty"`
	Host    string `json:"host,omitempty"`
	Port    int    `json:"port,omitempty"`
	Server  bool   `json:"server,omitempty"`
	NoWait  bool   `json:"no_wait,omitempty"`
	NoDelay bool   `json:"no_delay,omitempty"`
}

func (cd QemuHostCharDevTelnet) String() string {
	var ret strings.Builder

	if cd.Monitor {
		ret.WriteString("monitor:")
	}

	ret.WriteString(cd.Resource())

	if cd.Server {
		ret.WriteString(",server")
	}
	if cd.NoWait {
		ret.WriteString(",nowait")
	}
	if cd.NoDelay {
		ret.WriteString(",nodelay")
	}

	return ret.String()
}

func (cd QemuHostCharDevTelnet) Resource() string {
	if len(cd.Host) == 0 {
		// Cannot stringify telnet character device without host
		return ""
	} else if cd.Port <= 0 {
		// Cannot stringify telnet character device without port
		return ""
	}

	var ret strings.Builder

	ret.WriteString(cd.Host)
	ret.WriteString(":")
	ret.WriteString(strconv.Itoa(cd.Port))

	return ret.String()
}

func (cd QemuHostCharDevTelnet) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevTelnet.Connection")
}

type QemuHostCharDevWebsocket struct {
	Monitor bool   `json:"monitor,omitempty"`
	Host    string `json:"host,omitempty"`
	Port    int    `json:"port,omitempty"`
	NoWait  bool   `json:"no_wait,omitempty"`
	NoDelay bool   `json:"no_delay,omitempty"`
}

func (cd QemuHostCharDevWebsocket) String() string {
	var ret strings.Builder

	if cd.Monitor {
		ret.WriteString("monitor:")
	}

	ret.WriteString(cd.Resource())

	// Client mode is not supported.
	ret.WriteString(",server")

	if cd.NoWait {
		ret.WriteString(",nowait")
	}
	if cd.NoDelay {
		ret.WriteString(",nodelay")
	}

	return ret.String()
}

func (cd QemuHostCharDevWebsocket) Resource() string {
	if len(cd.Host) == 0 {
		// Cannot stringify websocket character device without host
		return ""
	} else if cd.Port <= 0 {
		// Cannot stringify websocket character device without port
		return ""
	}

	var ret strings.Builder

	ret.WriteString(cd.Host)
	ret.WriteString(":")
	ret.WriteString(strconv.Itoa(cd.Port))

	return ret.String()
}

func (cd QemuHostCharDevWebsocket) Connection() (net.Conn, error) {
	return nil, fmt.Errorf("not implemented: machine.qemu.QemuHostCharDevWebsocket.Connection")
}

type QemuHostCharDevUnix struct {
	Monitor   bool   `json:"monitor,omitempty"`
	SocketDir string `json:"socket_dir,omitempty"`
	Name      string `json:"name,omitempty"`
	Path      string `json:"path,omitempty"`
	Server    bool   `json:"server,omitempty"`
	NoWait    bool   `json:"no_wait,omitempty"`
	Reconnect int    `json:"reconnect,omitempty"`
}

func (cd QemuHostCharDevUnix) String() string {
	var ret strings.Builder

	if cd.Monitor {
		ret.WriteString("mon:")
	}

	ret.WriteString("unix:")
	ret.WriteString(cd.Resource())

	if cd.Server {
		ret.WriteString(",server")
	}
	if cd.NoWait {
		ret.WriteString(",nowait")
	}
	if cd.Reconnect > 0 {
		ret.WriteString(",reconnect=")
		ret.WriteString(strconv.Itoa(cd.Reconnect))
	}

	return ret.String()
}

func (cd QemuHostCharDevUnix) Resource() string {
	if len(cd.Path) == 0 && (len(cd.SocketDir) == 0 || len(cd.Name) == 0) {
		// Cannot stringify unix socket character device without path or socket dir
		return ""
	} else if len(cd.Path) == 0 {
		if ext := filepath.Ext(cd.Name); len(ext) == 0 {
			cd.Name += ".sock"
		}

		cd.Path = filepath.Join(cd.SocketDir, cd.Name)
	}

	return cd.Path
}

func (cd QemuHostCharDevUnix) Connection() (net.Conn, error) {
	return net.Dial("unix", cd.Resource())
}

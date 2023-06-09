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

type QemuCharDev interface {
	fmt.Stringer
}

type QemuCharDevType string

const (
	QemuCharDevTypeFile           = QemuCharDevType("file")
	QemuCharDevTypeNamed          = QemuCharDevType("chardev")
	QemuCharDevTypeNone           = QemuCharDevType("none")
	QemuCharDevTypeNull           = QemuCharDevType("null")
	QemuCharDevTypeParallel       = QemuCharDevType("parallel")
	QemuCharDevTypeParport        = QemuCharDevType("parport")
	QemuCharDevTypePipe           = QemuCharDevType("pipe")
	QemuCharDevTypePty            = QemuCharDevType("pty")
	QemuCharDevTypeRingBuf        = QemuCharDevType("ringbuf")
	QemuCharDevTypeSerial         = QemuCharDevType("serial")
	QemuCharDevTypeSocket         = QemuCharDevType("socket")
	QemuCharDevTypeSpicePort      = QemuCharDevType("spiceport")
	QemuCharDevTypeSpiceVMC       = QemuCharDevType("spicevmc")
	QemuCharDevTypeStdio          = QemuCharDevType("stdio")
	QemuCharDevTypeTCP            = QemuCharDevType("tcp")
	QemuCharDevTypeTelnet         = QemuCharDevType("telnet")
	QemuCharDevTypeTty            = QemuCharDevType("tty")
	QemuCharDevTypeUDP            = QemuCharDevType("udp")
	QemuCharDevTypeUnix           = QemuCharDevType("unix")
	QemuCharDevTypeVirtualConsole = QemuCharDevType("vc")
	QemuCharDevTypeWebsocket      = QemuCharDevType("websocket")
)

// QemuCharDevNull represents a null character device
type QemuCharDevNull struct {
	Id        string
	Multiplex bool
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// null,id=id[,mux=on|off][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevNull) String() string {
	if len(cd.Id) == 0 {
		// Cannot stringify null character device without id
		return ""
	}

	var ret strings.Builder

	ret.WriteString(string(QemuCharDevTypeNull))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)

	if cd.Multiplex {
		ret.WriteString(",mux=on")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevNull represents a character device based on TCP socket
type QemuCharDevSocketTCP struct {
	Id        string
	Host      string
	Port      int
	To        string
	Ipv4      bool
	Ipv6      bool
	NoDelay   bool
	Reconnect int
	Server    bool
	NoWait    bool
	WebSocket bool
	Multiplex bool
	LogFile   string
	LogAppend bool
	TLSCreds  string
	TLSAuthz  string
}

// String returns a QEMU command-line compatible chardev string with the format:
// socket,id=id[,host=host],port=port[,to=to][,ipv4][,ipv6][,nodelay]
// [,reconnect=seconds][,server][,nowait][,telnet][,websocket]
// [,reconnect=seconds][,mux=on|off][,logfile=PATH][,logappend=on|off]
// [,tls-creds=ID][,tls-authz=ID]
func (cd QemuCharDevSocketTCP) String() string {
	if len(cd.Id) == 0 {
		// Cannot stringify character device without id
		return ""
	}

	var ret strings.Builder

	ret.WriteString(string(QemuCharDevTypeSocket))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)

	if len(cd.Host) > 0 {
		ret.WriteString(",host=")
		ret.WriteString(cd.Host)
	}
	if cd.Port > 0 {
		ret.WriteString(",port=")
		ret.WriteString(strconv.Itoa(cd.Port))
	}
	if len(cd.To) > 0 {
		ret.WriteString(",to=")
		ret.WriteString(cd.To)
	}
	if cd.Ipv4 {
		ret.WriteString(",ipv4")
	}
	if cd.Ipv6 {
		ret.WriteString(",ipv6")
	}
	if cd.NoDelay {
		ret.WriteString(",nodelay")
	}
	if cd.Reconnect > 0 {
		ret.WriteString(",reconnect=")
		ret.WriteString(strconv.Itoa(cd.Reconnect))
	}
	if cd.Server {
		ret.WriteString(",server")
	}
	if cd.NoWait {
		ret.WriteString(",nowait")
	}
	if cd.WebSocket {
		ret.WriteString(",websocket")
	}
	if cd.Multiplex {
		ret.WriteString(",mux=on")
	} else {
		ret.WriteString(",mux=off")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}
	if len(cd.TLSCreds) > 0 {
		ret.WriteString(",tlscreds=")
		ret.WriteString(cd.TLSCreds)
	}
	if len(cd.TLSAuthz) > 0 {
		ret.WriteString(",tlsauthz=")
		ret.WriteString(cd.TLSAuthz)
	}

	return ret.String()
}

// QemuCharDevNull represents a character device based on a UNIX socket
type QemuCharDevSocketUnix struct {
	Id        string
	Path      string
	Server    bool
	NoWait    bool
	Telnet    bool
	WebSocket bool
	Reconnect int
	Multiplex bool
	LogFile   string
	LogAppend bool
	Abstract  bool
	Tight     bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// socket,id=id,path=path[,server][,nowait][,telnet][,websocket]
// [,reconnect=seconds][,mux=on|off][,logfile=PATH][,logappend=on|off]
// [,abstract=on|off][,tight=on|off]
func (cd QemuCharDevSocketUnix) String() string {
	if len(cd.Id) == 0 {
		// Cannot stringify character device without id
		return ""
	}

	var ret strings.Builder

	ret.WriteString(string(QemuCharDevTypeSocket))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)
	ret.WriteString(",path=")
	ret.WriteString(cd.Path)

	if cd.Server {
		ret.WriteString(",server")
	}
	if cd.NoWait {
		ret.WriteString(",nowait")
	}
	if cd.Telnet {
		ret.WriteString(",telnet")
	}
	if cd.WebSocket {
		ret.WriteString(",websocket")
	}
	if cd.Reconnect > 0 {
		ret.WriteString(",reconnect=")
		ret.WriteString(strconv.Itoa(cd.Reconnect))
	}
	if cd.Multiplex {
		ret.WriteString(",mux=on")
	} else {
		ret.WriteString(",mux=off")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}
	if cd.Abstract {
		ret.WriteString(",abstract=on")
	} else {
		ret.WriteString(",abstract=off")
	}
	if cd.Tight {
		ret.WriteString(",tight=on")
	} else {
		ret.WriteString(",tight=off")
	}

	return ret.String()
}

// QemuCharDevUdp represents a UDP-accessible character device based
type QemuCharDevUdp struct {
	Id        string
	Host      string
	Port      int
	LocalAddr string
	LocalPort int
	Ipv4      bool
	Ipv6      bool
	Multiplex bool
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// udp,id=id[,host=host],port=port[,localaddr=localaddr]
// [,localport=localport][,ipv4][,ipv6][,mux=on|off]
// [,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevUdp) String() string {
	if len(cd.Id) == 0 {
		// Cannot stringify character device without id
		return ""
	}

	var ret strings.Builder

	ret.WriteString(string(QemuCharDevTypeUDP))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)

	if len(cd.Host) > 0 {
		ret.WriteString(",host=")
		ret.WriteString(cd.Host)
	}
	if cd.Port > 0 {
		ret.WriteString(",port=")
		ret.WriteString(strconv.Itoa(cd.Port))
	}
	if len(cd.LocalAddr) > 0 {
		ret.WriteString(",localaddr=")
		ret.WriteString(cd.LocalAddr)
	}
	if cd.LocalPort > 0 {
		ret.WriteString(",localport=")
		ret.WriteString(strconv.Itoa(cd.LocalPort))
	}
	if cd.Ipv4 {
		ret.WriteString(",ipv4")
	}
	if cd.Ipv6 {
		ret.WriteString(",ipv6")
	}
	if cd.Multiplex {
		ret.WriteString(",mux=on")
	} else {
		ret.WriteString(",mux=off")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevVirtualConsole represents a virtual console character device
type QemuCharDevVirtualConsole struct {
	Id        string
	Width     int
	Height    int
	Cols      int
	Rows      int
	Multiplex bool
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// vc,id=id[[,width=width][,height=height]][[,cols=cols][,rows=rows]]
// [,mux=on|off][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevVirtualConsole) String() string {
	if len(cd.Id) == 0 {
		// Cannot stringify character device without id
		return ""
	}

	var ret strings.Builder

	ret.WriteString(string(QemuCharDevTypeVirtualConsole))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)

	if cd.Width > 0 && cd.Height > 0 {
		ret.WriteString(",width=")
		ret.WriteString(strconv.Itoa(cd.Width))
		ret.WriteString(",height=")
		ret.WriteString(strconv.Itoa(cd.Height))
	}
	if cd.Cols > 0 && cd.Rows > 0 {
		ret.WriteString(",cols=")
		ret.WriteString(strconv.Itoa(cd.Cols))
		ret.WriteString(",rows=")
		ret.WriteString(strconv.Itoa(cd.Rows))
	}
	if cd.Multiplex {
		ret.WriteString(",mux=on")
	} else {
		ret.WriteString(",mux=off")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevRingBuf represents a character device based on a ring buffer
type QemuCharDevRingBuf struct {
	Id        string
	Size      int
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// ringbuf,id=id[,size=size][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevRingBuf) String() string {
	if len(cd.Id) == 0 {
		// Cannot stringify character device without id
		return ""
	}

	var ret strings.Builder

	ret.WriteString(string(QemuCharDevTypeRingBuf))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)

	if cd.Size > 0 {
		ret.WriteString(",size=")
		ret.WriteString(strconv.Itoa(cd.Size))
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevFile represents a file character device
type QemuCharDevFile struct {
	Id        string
	Path      string
	Multiplex bool
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// file,id=id,path=path[,mux=on|off][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevFile) String() string {
	if len(cd.Id) == 0 || len(cd.Path) == 0 {
		// Cannot stringify character device without id or path
		return ""
	}

	var ret strings.Builder

	ret.WriteString(string(QemuCharDevTypeFile))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)
	ret.WriteString(",path=")
	ret.WriteString(cd.Path)

	if cd.Multiplex {
		ret.WriteString(",mux=on")
	} else {
		ret.WriteString(",mux=off")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevPipe represents a character device based on a pipe
type QemuCharDevPipe struct {
	Id        string
	Path      string
	Multiplex bool
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// pipe,id=id,path=path[,mux=on|off][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevPipe) String() string {
	if len(cd.Id) == 0 || len(cd.Path) == 0 {
		// Cannot stringify character device without id or path
		return ""
	}

	var ret strings.Builder
	ret.WriteString(string(QemuCharDevTypePipe))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)
	ret.WriteString(",path=")
	ret.WriteString(cd.Path)

	if cd.Multiplex {
		ret.WriteString(",mux=on")
	} else {
		ret.WriteString(",mux=off")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevPty represents a character device based on PTY
type QemuCharDevPty struct {
	Id        string
	Multiplex bool
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// pty,id=id[,mux=on|off][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevPty) String() string {
	if len(cd.Id) == 0 {
		// Cannot stringify character device without id
		return ""
	}

	var ret strings.Builder
	ret.WriteString(string(QemuCharDevTypePty))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)

	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevStdio represents a character device based on stdio
type QemuCharDevStdio struct {
	Id        string
	Multiplex bool
	Signal    bool
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// stdio,id=id[,mux=on|off][,signal=on|off][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevStdio) String() string {
	if len(cd.Id) == 0 {
		// Cannot stringify character device without id
		return ""
	}

	var ret strings.Builder
	ret.WriteString(string(QemuCharDevTypeStdio))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)

	if cd.Multiplex {
		ret.WriteString(",mux=on")
	} else {
		ret.WriteString(",mux=off")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevSerial represents a character device based on serial device
type QemuCharDevSerial struct {
	Id        string
	Path      string
	Multiplex bool
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// serial,id=id,path=path[,mux=on|off][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevSerial) String() string {
	if len(cd.Id) == 0 || len(cd.Path) == 0 {
		// Cannot stringify character device without id or path
		return ""
	}

	var ret strings.Builder
	ret.WriteString(string(QemuCharDevTypeSerial))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)
	ret.WriteString(",path=")
	ret.WriteString(cd.Path)

	if cd.Multiplex {
		ret.WriteString(",mux=on")
	} else {
		ret.WriteString(",mux=off")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevTty represents a character device based on a TTY
type QemuCharDevTty struct {
	Id        string
	Path      string
	Multiplex bool
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// tty,id=id,path=path[,mux=on|off][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevTty) String() string {
	if len(cd.Id) == 0 || len(cd.Path) == 0 {
		// Cannot stringify character device without id or path
		return ""
	}

	var ret strings.Builder
	ret.WriteString(string(QemuCharDevTypeTty))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)
	ret.WriteString(",path=")
	ret.WriteString(cd.Path)

	if cd.Multiplex {
		ret.WriteString(",mux=on")
	} else {
		ret.WriteString(",mux=off")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevParallel represents a parallel character device
type QemuCharDevParallel struct {
	Id        string
	Path      string
	Multiplex bool
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// parallel,id=id,path=path[,mux=on|off][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevParallel) String() string {
	if len(cd.Id) == 0 || len(cd.Path) == 0 {
		// Cannot stringify character device without id or path
		return ""
	}

	var ret strings.Builder
	ret.WriteString(string(QemuCharDevTypeParallel))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)
	ret.WriteString(",path=")
	ret.WriteString(cd.Path)

	if cd.Multiplex {
		ret.WriteString(",mux=on")
	} else {
		ret.WriteString(",mux=off")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevParport represents a character device based on parport
type QemuCharDevParport struct {
	Id        string
	Path      string
	Multiplex bool
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// parport,id=id,path=path[,mux=on|off][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevParport) String() string {
	if len(cd.Id) == 0 || len(cd.Path) == 0 {
		// Cannot stringify character device without id or path
		return ""
	}

	var ret strings.Builder
	ret.WriteString(string(QemuCharDevTypeParport))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)
	ret.WriteString(",path=")
	ret.WriteString(cd.Path)

	if cd.Multiplex {
		ret.WriteString(",mux=on")
	} else {
		ret.WriteString(",mux=off")
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevSpiceVMC represents a character device based on spice VMC
type QemuCharDevSpiceVMC struct {
	Id        string
	Name      string
	Debug     string
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// spicevmc,id=id,name=name[,debug=debug][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevSpiceVMC) String() string {
	if len(cd.Id) == 0 || len(cd.Name) == 0 {
		// Cannot stringify character device without id or name
		return ""
	}

	var ret strings.Builder
	ret.WriteString(string(QemuCharDevTypeSpiceVMC))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)
	ret.WriteString(",name=")
	ret.WriteString(cd.Name)

	if len(cd.Debug) > 0 {
		ret.WriteString(",debug=")
		ret.WriteString(cd.Debug)
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

// QemuCharDevSpicePort represents a character device based on a spice port
type QemuCharDevSpicePort struct {
	Id        string
	Name      string
	Debug     string
	LogFile   string
	LogAppend bool
}

// String returns a QEMU command-line compatible chardev string with the format:
// spiceport,id=id,name=name[,debug=debug][,logfile=PATH][,logappend=on|off]
func (cd QemuCharDevSpicePort) String() string {
	if len(cd.Id) == 0 || len(cd.Name) == 0 {
		// Cannot stringify character device without id or name
		return ""
	}

	var ret strings.Builder
	ret.WriteString(string(QemuCharDevTypeSpicePort))
	ret.WriteString(",id=")
	ret.WriteString(cd.Id)
	ret.WriteString(",name=")
	ret.WriteString(cd.Name)

	if len(cd.Debug) > 0 {
		ret.WriteString(",debug=")
		ret.WriteString(cd.Debug)
	}
	if len(cd.LogFile) > 0 {
		ret.WriteString(",logfile=")
		ret.WriteString(cd.LogFile)

		if cd.LogAppend {
			ret.WriteString(",logappend=on")
		} else {
			ret.WriteString(",logappend=off")
		}
	}

	return ret.String()
}

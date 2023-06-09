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

type QemuNetDev interface {
	fmt.Stringer
}

type QemuNetDevType string

const (
	QemuNetDevTypeBridge    = QemuNetDevType("bridge")
	QemuNetDevTypeHubport   = QemuNetDevType("hubport")
	QemuNetDevTypeL2tpv3    = QemuNetDevType("l2tpv3")
	QemuNetDevTypeSocket    = QemuNetDevType("socket")
	QemuNetDevTypeTap       = QemuNetDevType("tap")
	QemuNetDevTypeUser      = QemuNetDevType("user")
	QemuNetDevTypeVde       = QemuNetDevType("vde")
	QemuNetDevTypeVhostUser = QemuNetDevType("vhost-user")
	QemuNetDevTypeVhostVdpa = QemuNetDevType("vhost-vdpa")
)

// Configure a host TAP network backend with ID that is connected to a bridge
// Br.
type QemuNetDevBridge struct {
	// ID of the network device.
	Id string
	// Bridge (default=br0)
	Br string
	// Use a program helper (default=/usr/lib/qemu/qemu-bridge-helper)
	Helper string
}

// String returns a QEMU command-line compatible netdev string with the format:
// bridge,id=str[,br=bridge][,helper=helper]
func (nd QemuNetDevBridge) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuNetDevTypeBridge))
	ret.WriteString(",id=")
	ret.WriteString(nd.Id)

	if len(nd.Br) > 0 {
		ret.WriteString(",br=")
		ret.WriteString(nd.Br)
	}
	if len(nd.Helper) > 0 {
		ret.WriteString(",helper=")
		ret.WriteString(nd.Helper)
	}

	return ret.String()
}

type QemuNetDevHubport struct {
	// ID of the network device.
	Id     string
	Hubid  string
	Netdev string
}

// String returns a QEMU command-line compatible netdev string with the format:
// hubport,id=str,hubid=n[,netdev=nd]
func (nd QemuNetDevHubport) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuNetDevTypeHubport))
	ret.WriteString(",id=")
	ret.WriteString(nd.Id)
	ret.WriteString(",hubid=")
	ret.WriteString(nd.Hubid)

	if len(nd.Netdev) > 0 {
		ret.WriteString(",netdev=")
		ret.WriteString(nd.Netdev)
	}

	return ret.String()
}

// Configure a network backend with ID connected to an Ethernet over L2TPv3
// pseudowire. Linux kernel 3.3+ as well as most routers can talk L2TPv3. This
// transport allows connecting a VM to a VM, VM to a router and even VM to Host.
// It is a nearly-universal standard (RFC3931). Note - this implementation uses
// static pre-configured tunnels (same as the Linux kernel).
type QemuNetDevL2tpv3 struct {
	// ID of the network device.
	Id string `json:"id,omitempty"`
	// Source address
	Src string `json:"src,omitempty"`
	// Destination address
	Dst string `json:"dst,omitempty"`
	// Specify source UDP port
	Srcport string `json:"srcport,omitempty"`
	// Destination UDP port
	Dstport   string `json:"dstport,omitempty"`
	Rxsession string `json:"rxsession,omitempty"`
	Txsession string `json:"txsession,omitempty"`
	// Force IPv6
	Ipv6 bool `json:"ipv6,omitempty"`
	// Enable UDP encapsulation
	Udp bool `json:"udp,omitempty"`
	// L2TPv3 uses cookies to prevent misconfiguration as well as a weak security
	// measure. Use Cookie64 to set cookie size to 64 bit, otherwise 32
	Cookie64 bool `json:"cookie64,omitempty"`
	// Force a 'cut-down' L2TPv3 with no counter
	Counter bool `json:"counter,omitempty"`
	// Work around broken counter handling in peer
	Pincounter bool `json:"pincounter,omitempty"`
	// Specify a txcookie (e.g. "0x12345678")
	Txcookie string `json:"txcookie,omitempty"`
	// Specify a rxcookie (e.g. "0x12345678")
	Rxcookie string `json:"rxcookie,omitempty"`
	// Add an extra offset between header and data
	Offset int `json:"offset,omitempty"`
}

// String returns a QEMU command-line compatible netdev string with the format:
// l2tpv3,id=str,src=srcaddr,dst=dstaddr[,srcport=srcport][,dstport=dstport]
// [,rxsession=rxsession],txsession=txsession[,ipv6=on/off][,udp=on/off]
// [,cookie64=on/off][,counter][,pincounter][,txcookie=txcookie]
// [,rxcookie=rxcookie][,offset=offset]
func (nd QemuNetDevL2tpv3) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuNetDevTypeL2tpv3))
	ret.WriteString(",id=")
	ret.WriteString(nd.Id)
	ret.WriteString(",src=")
	ret.WriteString(nd.Src)
	ret.WriteString(",dst=")
	ret.WriteString(nd.Dst)

	if len(nd.Srcport) > 0 {
		ret.WriteString(",srcport=")
		ret.WriteString(nd.Srcport)
	}
	if len(nd.Dstport) > 0 {
		ret.WriteString(",dstport=")
		ret.WriteString(nd.Dstport)
	}
	if len(nd.Rxsession) > 0 {
		ret.WriteString(",rxsession=")
		ret.WriteString(nd.Rxsession)
	}
	if len(nd.Txsession) > 0 {
		ret.WriteString(",txsession=")
		ret.WriteString(nd.Txsession)
	}
	if nd.Ipv6 {
		ret.WriteString(",ipv6=on")
	}
	if nd.Udp {
		ret.WriteString(",udp=on")
	}
	if nd.Cookie64 {
		ret.WriteString(",cookie64=on")
	}
	if nd.Counter {
		ret.WriteString(",counter=on")
	}
	if nd.Pincounter {
		ret.WriteString(",pincounter=on")
	}
	if len(nd.Txcookie) > 0 {
		ret.WriteString(",txcookie=")
		ret.WriteString(nd.Txcookie)
	}
	if len(nd.Rxcookie) > 0 {
		ret.WriteString(",rxcookie=")
		ret.WriteString(nd.Rxcookie)
	}
	if nd.Offset > 0 {
		ret.WriteString(",offset=")
		ret.WriteString(strconv.Itoa(nd.Offset))
	}

	return ret.String()
}

type QemuNetDevSocket struct {
	// ID of the network device.
	Id string `json:"id,omitempty"`
	// File descriptor ID.
	Fd int `json:"fd,omitempty"`
	// Listen address.
	Listen string `json:"listen,omitempty"`
	// Configure a network backend to connect to another network using a socket
	// connection.
	Connect string `json:"connect,omitempty"`
	// Configure a network backend to connect to a multicast maddr and port.
	Mcast string `json:"mcast,omitempty"`
	// Configure a network backend to connect to another network using an UDP
	// tunnel.
	Udp string `json:"udp,omitempty"`
	// SPecify the host address to send packages from.
	Localaddr string `json:"localaddr,omitempty"`
}

// String returns a QEMU command-line compatible netdev string with the format:
// socket,id=str[,fd=h][,listen=[host]:port][,connect=host:port]
// [,mcast=maddr:port[,localaddr=addr]][,udp=host:port][,localaddr=host:port]
func (nd QemuNetDevSocket) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuNetDevTypeSocket))
	ret.WriteString(",id=")
	ret.WriteString(nd.Id)

	if nd.Fd > 0 {
		ret.WriteString(",fd=")
		ret.WriteString(strconv.Itoa(nd.Fd))
	}
	if len(nd.Listen) > 0 {
		ret.WriteString(",listen=")
		ret.WriteString(nd.Listen)
	}
	if len(nd.Connect) > 0 {
		ret.WriteString(",connect=")
		ret.WriteString(nd.Connect)
	}
	if len(nd.Mcast) > 0 {
		ret.WriteString(",mcast=")
		ret.WriteString(nd.Mcast)
	}
	if len(nd.Udp) > 0 {
		ret.WriteString(",udp=")
		ret.WriteString(nd.Udp)
	}
	if len(nd.Localaddr) > 0 {
		ret.WriteString(",localaddr=")
		ret.WriteString(nd.Localaddr)
	}

	return ret.String()
}

// Configure a host TAP network backend
type QemuNetDevTap struct {
	// ID of the network device.
	Id string `json:"id,omitempty"`
	// Connect to an already opened TAP interface
	Fd int `json:"fd,omitempty"`
	// Connect to already opened multiqueue capable TAP interfaces.
	Fds []int `json:"fds,omitempty"`
	// Interface name
	Ifname string `json:"ifname,omitempty"`
	// Use script (default=/etc/qemu-ifup) to configure it and value of 'no' to
	// disable execution.
	Script string `json:"script,omitempty"`
	// Use downscript (default=/etc/qemu-ifdown) to deconfigure it and value of
	// 'no' to disable execution.
	Downscript string `json:"downscript,omitempty"`
	// Connected to a bridge (default=br0)
	Br string `json:"br,omitempty"`
	// Use network helper script (default=/usr/lib/qemu/qemu-bridge-helper) to
	// configure it
	Helper string `json:"helper,omitempty"`
	// Limit the size of the send buffer (the default is disabled 'sndbuf=0' to
	// enable flow control set 'sndbuf=1048576')
	Sndbuf int `json:"sndbuf,omitempty"`
	// Set to false to avoid enabling the IFF_VNET_HDR tap flag and true to make
	// the lack of IFF_VNET_HDR support an error condition
	VnetHdr bool `json:"vnet-hdr,omitempty"`
	// Enable experimental in kernel accelerator (only has effect for virtio
	// guests which use MSIX)
	Vhost bool `json:"vhost,omitempty"`
	// Connect to an already opened vhost net device.
	Vhostfd int `json:"vhostfd,omitempty"`
	// Connect to multiple already opened vhost net devices.
	Vhostfds []int `json:"vhostfds,omitempty"`
	// Set to true to force vhost on for non-MSIX virtio guests.
	Vhostforce bool `json:"vhostforce,omitempty"`
	// Specify the number of queues to be created for multiqueue TAP.
	Queues int `json:"queues,omitempty"`
	// Specify the maximum number of microseconds that could be spent on busy
	// polling for vhost net.
	PollUs int `json:"poll-us,omitempty"`
}

// String returns a QEMU command-line compatible netdev string with the format:
// tap,id=str[,fd=h][,fds=x:y:...:z][,ifname=name][,script=file]
// [,downscript=dfile][,br=bridge][,helper=helper][,sndbuf=nbytes]
// [,vnet_hdr=on|off][,vhost=on|off][,vhostfd=h][,vhostfds=x:y:...:z]
// [,vhostforce=on|off][,queues=n][,poll-us=n]
func (nd QemuNetDevTap) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuNetDevTypeTap))
	ret.WriteString(",id=")
	ret.WriteString(nd.Id)

	if nd.Fd > 0 {
		ret.WriteString(",fd=")
		ret.WriteString(strconv.Itoa(nd.Fd))
	}
	if len(nd.Fds) > 0 {
		ret.WriteString(",fds=")
		ret.WriteString(strings.Trim(strings.Replace(fmt.Sprint(nd.Fds), " ", ":", -1), "[]"))
	}
	if len(nd.Ifname) > 0 {
		ret.WriteString(",ifname=")
		ret.WriteString(nd.Ifname)
	}
	if len(nd.Script) > 0 {
		ret.WriteString(",script=")
		ret.WriteString(nd.Script)
	}
	if len(nd.Downscript) > 0 {
		ret.WriteString(",downscript=")
		ret.WriteString(nd.Downscript)
	}
	if len(nd.Br) > 0 {
		ret.WriteString(",br=")
		ret.WriteString(nd.Br)
	}
	if len(nd.Helper) > 0 {
		ret.WriteString(",helper=")
		ret.WriteString(nd.Helper)
	}
	if nd.Sndbuf > 0 {
		ret.WriteString(",sndbuf=")
		ret.WriteString(strconv.Itoa(nd.Sndbuf))
	}
	if nd.VnetHdr {
		ret.WriteString(",vnet_hdr=on")
	}
	if nd.Vhost {
		ret.WriteString(",vhost=on")
	}
	if nd.Vhostfd > 0 {
		ret.WriteString(",vhostfd=")
		ret.WriteString(strconv.Itoa(nd.Vhostfd))
	}
	if len(nd.Vhostfds) > 0 {
		ret.WriteString(",vhostfds=")
		ret.WriteString(strings.Trim(strings.Replace(fmt.Sprint(nd.Vhostfds), " ", ":", -1), "[]"))
	}
	if nd.Vhostforce {
		ret.WriteString(",vhostforce=on")
	}
	if nd.Queues > 0 {
		ret.WriteString(",queues=")
		ret.WriteString(strconv.Itoa(nd.Queues))
	}
	if nd.PollUs > 0 {
		ret.WriteString(",poll-us=")
		ret.WriteString(strconv.Itoa(nd.PollUs))
	}

	return ret.String()
}

// Configure a user mode network backend.
type QemuNetDevUser struct {
	// ID of the network device.
	Id             string `json:"id,omitempty"`
	Ipv4           bool   `json:"ipv4,omitempty"`
	Net            string `json:"net,omitempty"`
	Host           string `json:"host,omitempty"`
	Ipv6           bool   `json:"ipv6,omitempty"`
	Ipv6Net        string `json:"ipv6-net,omitempty"`
	Ipv6Host       string `json:"ipv6-host,omitempty"`
	Restrict       bool   `json:"restrict,omitempty"`
	Hostname       string `json:"hostname,omitempty"`
	Domainname     string `json:"domainname,omitempty"`
	Tftp           string `json:"tftp,omitempty"`
	TftpServerName string `json:"tftp_server_name,omitempty"`
	Bootfile       string `json:"bootfile,omitempty"`
	Hostfwd        string `json:"hostfwd,omitempty"`
	Guestfwd       string `json:"guestfwd,omitempty"`
	Smb            string `json:"smb,omitempty"`
	Smbserver      string `json:"smbserver,omitempty"`
}

// String returns a QEMU command-line compatible netdev string with the format:
// user,id=str[,ipv4[=on|off]][,net=addr[/mask]][,host=addr][,ipv6[=on|off]]
// [,ipv6-net=addr[/int]][,ipv6-host=addr][,restrict=on|off][,hostname=host]
// [,dhcpstart=addr][,dns=addr][,ipv6-dns=addr][,dnssearch=domain]
// [,domainname=domain][,tftp=dir][,tftp-server-name=name][,bootfile=f]
// [,hostfwd=rule][,guestfwd=rule][,smb=dir[,smbserver=addr]]
func (nd QemuNetDevUser) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuNetDevTypeUser))
	ret.WriteString(",id=")
	ret.WriteString(nd.Id)

	if nd.Ipv4 {
		ret.WriteString(",ipv4=on")
	}
	if len(nd.Net) > 0 {
		ret.WriteString(",net=")
		ret.WriteString(nd.Net)
	}
	if len(nd.Host) > 0 {
		ret.WriteString(",host=")
		ret.WriteString(nd.Host)
	}
	if nd.Ipv6 {
		ret.WriteString(",ipv6=on")
	}
	if len(nd.Ipv6Net) > 0 {
		ret.WriteString(",ipv6-net=")
		ret.WriteString(nd.Ipv6Net)
	}
	if len(nd.Ipv6Host) > 0 {
		ret.WriteString(",ipv6-host=")
		ret.WriteString(nd.Ipv6Host)
	}
	if nd.Restrict {
		ret.WriteString(",restrict=on")
	}
	if len(nd.Hostname) > 0 {
		ret.WriteString(",hostname=")
		ret.WriteString(nd.Hostname)
	}
	if len(nd.Domainname) > 0 {
		ret.WriteString(",domainname=")
		ret.WriteString(nd.Domainname)
	}
	if len(nd.Tftp) > 0 {
		ret.WriteString(",tftp=")
		ret.WriteString(nd.Tftp)
	}
	if len(nd.TftpServerName) > 0 {
		ret.WriteString(",tftp-server-name=")
		ret.WriteString(nd.TftpServerName)
	}
	if len(nd.Bootfile) > 0 {
		ret.WriteString(",bootfile=")
		ret.WriteString(nd.Bootfile)
	}
	if len(nd.Hostfwd) > 0 {
		ret.WriteString(",hostfwd=")
		ret.WriteString(nd.Hostfwd)
	}
	if len(nd.Guestfwd) > 0 {
		ret.WriteString(",guestfwd=")
		ret.WriteString(nd.Guestfwd)
	}
	if len(nd.Smb) > 0 {
		ret.WriteString(",smb=")
		ret.WriteString(nd.Smb)
	}
	if len(nd.Smbserver) > 0 {
		ret.WriteString(",smbserver=")
		ret.WriteString(nd.Smbserver)
	}

	return ret.String()
}

type QemuNetDevVde struct {
	// ID of the network device.
	Id string `json:"id,omitempty"`
	// Host of the VDE switch.
	Socket string `json:"socket,omitempty"`
	// Connect to port.
	Port int `json:"port,omitempty"`
	// Change default ownership for communication port.
	Group string `json:"group,omitempty"`
	// Change default permissions for communication port.
	Mode string `json:"mode,omitempty"`
}

// String returns a QEMU command-line compatible netdev string with the format:
// vde,id=str[,sock=socketpath][,port=n][,group=groupname][,mode=octalmode]
func (nd QemuNetDevVde) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuNetDevTypeVde))
	ret.WriteString(",id=")
	ret.WriteString(nd.Id)

	if len(nd.Socket) > 0 {
		ret.WriteString(",sock=")
		ret.WriteString(nd.Socket)
	}
	if nd.Port > 0 {
		ret.WriteString(",port=")
		ret.WriteString(strconv.Itoa(nd.Port))
	}
	if len(nd.Group) > 0 {
		ret.WriteString(",group=")
		ret.WriteString(nd.Group)
	}
	if len(nd.Mode) > 0 {
		ret.WriteString(",mode=")
		ret.WriteString(nd.Mode)
	}

	return ret.String()
}

type QemuNetDevVhostUser struct {
	// ID of the network device.
	Id string `json:"id,omitempty"`
	// Character device backend.
	Chardev string `json:"chardev,omitempty"`
	// Force vhost-user network.
	Vhostforce bool `json:"vhostforce,omitempty"`
}

// String returns a QEMU command-line compatible netdev string with the format:
// vhost-user,id=str,chardev=dev[,vhostforce=on|off]
func (nd QemuNetDevVhostUser) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuNetDevTypeVhostUser))
	ret.WriteString(",id=")
	ret.WriteString(nd.Id)
	ret.WriteString(",chardev=")
	ret.WriteString(nd.Chardev)

	if nd.Vhostforce {
		ret.WriteString(",vhostforce=on")
	}

	return ret.String()
}

// Establish a vhost-vdpa netdev
type QemuNetDevVhostVdpa struct {
	// ID of the network device.
	Id string `json:"id,omitempty"`
	// Vhost VDPA network
	Vhostdev string `json:"vhostdev,omitempty"`
}

// String returns a QEMU command-line compatible netdev string with the format:
// vhost-vdpa,id=str,vhostdev=/path/to/dev
func (nd QemuNetDevVhostVdpa) String() string {
	var ret strings.Builder

	ret.WriteString(string(QemuNetDevTypeVhostVdpa))
	ret.WriteString(",id=")
	ret.WriteString(nd.Id)
	ret.WriteString(",vhostdev=")
	ret.WriteString(nd.Vhostdev)

	return ret.String()
}

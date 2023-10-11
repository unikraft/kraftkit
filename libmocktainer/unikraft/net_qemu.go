// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package unikraft

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// SetupQemuNet sets up the network for QEMU within the current network
// namespace, and returns QEMU network flags for the created netlink.
func SetupQemuNet() (qemuNetArgs []string, _ error) {
	defRoute, err := defaultRoute()
	if err != nil {
		return nil, fmt.Errorf("getting default route: %w", err)
	}
	if defRoute == nil {
		// no container network
		return nil, nil
	}

	defNetlinkIdx := defRoute.LinkIndex
	defNetlink, err := netlink.LinkByIndex(defNetlinkIdx)
	if err != nil {
		return nil, fmt.Errorf("getting default netlink with index %d: %w", defNetlinkIdx, err)
	}

	defNetlinkAddr, err := netlinkAddress(defNetlink)
	if err != nil {
		return nil, fmt.Errorf("getting addr of default netlink %s: %w", defNetlink.Attrs().Name, err)
	}
	if defNetlinkAddr == nil {
		// no container network
		return nil, nil
	}

	const tapName = "uktap0"
	mvt, err := createBridgeMacvtap(tapName, defNetlinkIdx)
	if err != nil {
		return nil, fmt.Errorf("creating macvtap netlink %s: %w", tapName, err)
	}

	tapDev := tapDevicePath(mvt)
	tapDevFd, err := unix.Open(tapDev, unix.O_RDWR, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open macvtap device", Path: tapDev, Err: err}
	}

	args := append(
		genQemuKernelNetCmdline(defNetlinkAddr.IP, defRoute.Gw, defNetlinkAddr.Mask),
		genQemuNetArgs(tapDevFd, mvt.HardwareAddr)...,
	)

	if err = netlink.AddrDel(defNetlink, defNetlinkAddr); err != nil {
		return nil, fmt.Errorf("deleting addr of default netlink %s: %w", tapName, err)
	}

	return args, nil
}

// genQemuNetArgs returns QEMU network flags for the given MacVTap netlink.
// devFd is the number of the file descriptor corresponding to the netlink's
// device, as passed to the QEMU process.
func genQemuNetArgs(devFd int, hwaddr net.HardwareAddr) []string {
	return []string{
		"-netdev", "tap,id=n0,fd=" + strconv.Itoa(devFd),
		"-device", "virtio-net-pci,netdev=n0,mac=" + hwaddr.String(),
	}
}

// genQemuKernelNetCmdline returns network arguments to be passed on the kernel's
// command line.
func genQemuKernelNetCmdline(addr, gwaddr net.IP, subnet net.IPMask) []string {
	var cmdline strings.Builder
	cmdline.WriteString("netdev.ipv4_addr=" + addr.String())
	cmdline.WriteByte(' ')
	cmdline.WriteString("netdev.ipv4_gw_addr=" + gwaddr.String())
	cmdline.WriteByte(' ')
	cmdline.WriteString("netdev.ipv4_subnet_mask=" + net.IP(subnet).String())
	cmdline.WriteByte(' ')
	cmdline.WriteString("--")
	cmdline.WriteByte(' ')

	return []string{
		"-append", cmdline.String(),
	}
}

// createBridgeMacvtap creates a MacVTap netlink that is bridged with the
// netlink for the current namespace's default route.
func createBridgeMacvtap(name string, parentIdx int) (*netlink.Macvtap, error) {
	if _, err := netlink.LinkByName(name); err == nil {
		return nil, fmt.Errorf("netlink %s already exists", name)
	}

	mvt := newBridgeMacvtap(name, parentIdx)
	if err := netlink.LinkAdd(mvt); err != nil {
		return nil, fmt.Errorf("adding macvtap netlink %s: %w", mvt.Name, err)
	}
	if err := netlink.LinkSetUp(mvt); err != nil {
		return mvt, fmt.Errorf("enabling macvtap netlink %s: %w", mvt.Name, err)
	}

	l, err := netlink.LinkByName(mvt.Name) // refresh attributes such as hwaddr
	if err != nil {
		return mvt, fmt.Errorf("getting macvtap netlink %s: %w", mvt.Name, err)
	}

	return l.(*netlink.Macvtap), nil
}

// defaultRoute returns the default route for the current namespace.
func defaultRoute() (*netlink.Route, error) {
	defaultRouteFilter := &netlink.Route{Dst: nil}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, defaultRouteFilter, netlink.RT_FILTER_DST)
	if err != nil {
		return nil, fmt.Errorf("listing default net routes: %w", err)
	}
	if n := len(routes); n > 1 {
		return nil, fmt.Errorf("found more than one default net routes (%d)", n)
	}

	if len(routes) == 0 {
		return nil, nil
	}

	return &routes[0], nil
}

// netlinkAddress returns the address of the given netlink. If the netlink has
// multiple addresses, the first one is returned.
func netlinkAddress(l netlink.Link) (*netlink.Addr, error) {
	addrs, err := netlink.AddrList(l, netlink.FAMILY_V4) // no ipv6 support in Unikraft
	if err != nil {
		return nil, fmt.Errorf("getting addresses of netlink %s: %w", l.Attrs().Name, err)
	}
	if len(addrs) == 0 {
		return nil, nil
	}

	return &addrs[0], nil
}

// newBridgeMacvtap initializes a netlink.Macvtap that operates in bridge mode
// with the given parent netlink.
func newBridgeMacvtap(name string, parentIdx int) *netlink.Macvtap {
	mvtAttr := netlink.NewLinkAttrs()
	mvtAttr.Name = name
	mvtAttr.ParentIndex = parentIdx

	return &netlink.Macvtap{
		Macvlan: netlink.Macvlan{
			LinkAttrs: mvtAttr,
			Mode:      netlink.MACVLAN_MODE_BRIDGE,
		},
	}
}

// tapDevicePath returns the path of the tap device for the given netlink.
func tapDevicePath(l *netlink.Macvtap) string {
	return "/dev/tap" + strconv.Itoa(l.Index)
}

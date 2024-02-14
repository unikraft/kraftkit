// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package bridge

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/erikh/ping"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	networkv1alpha1 "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/machine/network/macaddr"
)

type v1alpha1Network struct{}

func NewNetworkServiceV1alpha1(ctx context.Context, opts ...any) (networkv1alpha1.NetworkService, error) {
	return &v1alpha1Network{}, nil
}

// Create implements kraftkit.sh/api/network/v1alpha1.Create
func (service *v1alpha1Network) Create(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	if network.Name == "" {
		return nil, fmt.Errorf("cannot create network without name")
	}

	if network.ObjectMeta.UID == "" {
		network.ObjectMeta.UID = uuid.NewUUID()
	}

	if network.Spec.IfName == "" {
		network.Spec.IfName = network.Name
	}

	network.Spec.Driver = "bridge"
	network.Status.State = networkv1alpha1.NetworkStateUnknown

	// Validate the options.
	if len(network.Spec.Gateway) == 0 {
		return network, fmt.Errorf("gateway cannot be empty")
	}
	if len(network.Spec.Netmask) == 0 {
		return network, fmt.Errorf("netmask cannot be empty")
	}

	bridge := &netlink.Bridge{
		LinkAttrs: netlink.NewLinkAttrs(),
	}

	bridge.LinkAttrs.MTU = DefaultMTU

	_, err := net.InterfaceByName(network.Name)
	if err == nil {
		// Bridge already exists, return early.
		return network, fmt.Errorf("network already exists: %s", network.Name)
	} else if !strings.Contains(err.Error(), "no such network interface") {
		return network, fmt.Errorf("getting interface %s failed: %v", network.Name, err)
	}

	// Create the bridge.
	la := netlink.NewLinkAttrs()
	la.Name = network.Name
	la.MTU = bridge.MTU
	br := &netlink.Bridge{LinkAttrs: la}
	if err := netlink.LinkAdd(br); err != nil {
		return network, fmt.Errorf("bridge creation for %s failed: %v", network.Name, err)
	}

	// br.Promisc = 1 // TODO(nderjung): Should the bridge be promiscuous?

	// Setup IP address for bridge.
	addr := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   net.ParseIP(network.Spec.Gateway),
			Mask: net.ParseIP(network.Spec.Netmask).DefaultMask(),
		},
	}
	if err := netlink.AddrAdd(br, addr); err != nil {
		return network, fmt.Errorf("adding address %s to bridge %s failed: %v", addr.String(), network.Name, err)
	}

	// Bring the bridge up.
	if err := netlink.LinkSetUp(br); err != nil {
		return network, fmt.Errorf("bringing bridge %s up failed: %v", network.Name, err)
	}

	network.CreationTimestamp = metav1.Now()

	link, err := netlink.LinkByName(network.Name)
	if err != nil {
		return network, fmt.Errorf("could not get bridge %s details: %v", network.Name, err)
	}

	// Use the internal network bridge networking system to determine
	// whether the identified network is online.
	if net.FlagUp&link.Attrs().Flags == 1 || net.FlagRunning&link.Attrs().Flags == 1 {
		network.Status.State = networkv1alpha1.NetworkStateUp
	} else {
		// TODO(nderjung): The bridge interface could be in other states.  For now
		// v1alpha1.NetworkState does not support more complex states, simply
		// indicate that it is "down" and therefore unusable.
		network.Status.State = networkv1alpha1.NetworkStateDown
	}

	// Add any interfaces
	for i, iface := range network.Spec.Interfaces {
		if iface.Spec.IfName == "" {
			j := 0
			for {
				ifname := fmt.Sprintf("%s@if%d", network.Name, j)
				if _, err := netlink.LinkByName(ifname); err != nil && err.Error() == "Link not found" {
					iface.Spec.IfName = ifname
					break
				}
				j++
			}
		}

		tap := &netlink.Tuntap{
			LinkAttrs: netlink.NewLinkAttrs(),
			Mode:      netlink.TUNTAP_MODE_TAP,
		}

		tap.Name = iface.Spec.IfName
		tap.MasterIndex = bridge.Attrs().Index
		tap.HardwareAddr, err = net.ParseMAC(iface.Spec.MacAddress)
		if err != nil {
			return network, err
		}

		if err := netlink.LinkAdd(tap); err != nil {
			return network, err
		}

		if err := netlink.LinkSetAlias(tap, fmt.Sprintf("%s:%s", network.ObjectMeta.UID, iface.ObjectMeta.UID)); err != nil {
			return network, err
		}

		network.Spec.Interfaces[i] = iface
	}

	return network, nil
}

// Start implements kraftkit.sh/api/network/v1alpha1.Start
func (service *v1alpha1Network) Start(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	// First, take down all interfaces
	for _, iface := range network.Spec.Interfaces {
		link, err := netlink.LinkByName(iface.Spec.IfName)
		if err != nil {
			return network, fmt.Errorf("getting link %s failed: %v", network.Name, err)
		}

		if err := netlink.LinkSetUp(link); err != nil {
			return network, fmt.Errorf("could not bring %s link down: %v", network.Name, err)
		}
	}

	// Get the bridge link.
	link, err := netlink.LinkByName(network.Name)
	if err != nil {
		return network, fmt.Errorf("getting bridge %s failed: %v", network.Name, err)
	}

	// Bring up the bridge link
	if err := netlink.LinkSetUp(link); err != nil {
		return network, fmt.Errorf("could not bring %s link up: %v", network.Name, err)
	}

	network.Status.State = networkv1alpha1.NetworkStateUp

	return network, nil
}

// Stop implements kraftkit.sh/api/network/v1alpha1.Stop
func (service *v1alpha1Network) Stop(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	// First, take down all interfaces
	for _, iface := range network.Spec.Interfaces {
		link, err := netlink.LinkByName(iface.Spec.IfName)
		if err != nil {
			return network, fmt.Errorf("getting link %s failed: %v", iface.Spec.IfName, err)
		}

		ip, _, err := net.ParseCIDR(iface.Spec.CIDR)
		if err != nil {
			return network, fmt.Errorf("could not parse IP address: %v", err)
		}

		if ping.Ping(&net.IPAddr{IP: ip, Zone: ""}, 150*time.Millisecond) {
			return network, fmt.Errorf("interface still in use: %s (%s, %s)", iface.Spec.IfName, iface.Spec.MacAddress, ip)
		}

		if err := netlink.LinkSetDown(link); err != nil {
			return network, fmt.Errorf("could not bring %s link down: %v", iface.Spec.IfName, err)
		}
	}

	// Get the bridge link.
	link, err := netlink.LinkByName(network.Name)
	if err != nil {
		return network, fmt.Errorf("getting bridge %s failed: %v", network.Name, err)
	}

	// Bring down the bridge link
	if err := netlink.LinkSetDown(link); err != nil {
		return network, fmt.Errorf("could not bring %s bridge down: %v", network.Name, err)
	}

	network.Status.State = networkv1alpha1.NetworkStateDown

	return network, nil
}

// Update implements kraftkit.sh/api/network/v1alpha1.Update.  This method only
// supports updating any provided
func (service *v1alpha1Network) Update(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	link, err := netlink.LinkByName(network.Name)
	if err != nil {
		return network, fmt.Errorf("could not get bridge link: %v", err)
	}

	bridge, ok := link.(*netlink.Bridge)
	if !ok {
		return network, fmt.Errorf("network link is not bridge")
	}

	bridgeface, err := net.InterfaceByName(bridge.Name)
	if err != nil {
		return nil, fmt.Errorf("could not get bridge interface: %v", err)
	}

	ipnet := &net.IPNet{
		IP:   net.ParseIP(network.Spec.Gateway),
		Mask: net.ParseIP(network.Spec.Netmask).DefaultMask(),
	}

	// Start MAC addresses iteratively.
	startMac, err := macaddr.GenerateMacAddress(true)
	if err != nil {
		return network, fmt.Errorf("could not prepare MAC address generator: %v", err)
	}

	// Populate a hashmap of link aliases that allow us to quickly reference later
	// on when we're clearing up unused interfaces.
	inuse := make(map[string]bool)

	// Add any defined interfaces.
	for i, iface := range network.Spec.Interfaces {
		if iface.ObjectMeta.UID == "" {
			iface.ObjectMeta.UID = uuid.NewUUID()
		}

		if iface.Spec.IfName == "" {
			j := 0
			for {
				ifname := fmt.Sprintf("%s@if%d", network.Name, j)
				if _, err := netlink.LinkByName(ifname); err != nil && err.Error() == "Link not found" {
					iface.Spec.IfName = ifname
					break
				}
				j++
			}
		}

		if iface.ObjectMeta.CreationTimestamp == *new(metav1.Time) {
			iface.ObjectMeta.CreationTimestamp = metav1.Now()
		}

		var mac net.HardwareAddr
		if iface.Spec.MacAddress == "" {
			startMac = macaddr.IncrementMacAddress(startMac)
			mac = startMac
			iface.Spec.MacAddress = mac.String()
		}

		if iface.Spec.CIDR == "" {
			ip, err := AllocateIP(ctx, ipnet, bridgeface, bridge)
			if err != nil {
				return network, fmt.Errorf("could not allocate interface IP for %s: %v", iface.Spec.IfName, err)
			}

			sz, _ := net.IPMask(net.ParseIP(network.Spec.Netmask).To4()).Size()
			iface.Spec.CIDR = fmt.Sprintf("%s/%d", ip.String(), sz)
		}

		tap := &netlink.Tuntap{
			LinkAttrs: netlink.NewLinkAttrs(),
			Mode:      netlink.TUNTAP_MODE_TAP,
		}
		tap.HardwareAddr = mac
		tap.MasterIndex = bridge.Attrs().Index
		tap.Name = iface.Spec.IfName

		if existing, err := netlink.LinkByName(tap.Name); err == nil {
			if existing.Attrs().Flags&net.FlagRunning != 0 {
				if err = netlink.LinkSetDown(tap); err != nil {
					return network, fmt.Errorf("could not bring %s link down: %v", iface.Spec.IfName, err)
				}
				if err := netlink.LinkModify(tap); err != nil {
					return network, fmt.Errorf("could not update %s link: %v", iface.Spec.IfName, err)
				}
			}
		} else {
			if err := netlink.LinkAdd(tap); err != nil {
				return network, fmt.Errorf("could not create %s link: %v", iface.Spec.IfName, err)
			}
		}

		// Set the alias such that it can be referenced later as the unique
		// combination of the network and this interface.
		alias := fmt.Sprintf("%s:%s", network.ObjectMeta.UID, iface.ObjectMeta.UID)
		if err := netlink.LinkSetAlias(tap, alias); err != nil {
			return network, fmt.Errorf("could not set link alias: %v", err)
		}

		if err = netlink.LinkSetUp(tap); err != nil {
			return network, fmt.Errorf("could not bring %s link up: %v", iface.Spec.IfName, err)
		}

		inuse[alias] = true
		network.Spec.Interfaces[i] = iface
	}

	// Clean up any removed interfaces.  Re-check the link list.
	links, err := netlink.LinkList()
	if err != nil {
		return network, fmt.Errorf("could not gather list of existing links: %v", err)
	}

	for _, link := range links {
		tap, ok := link.(*netlink.Tuntap)
		if !ok {
			continue // Skip non-tap interfaces
		}

		if _, ok := inuse[tap.Alias]; ok {
			continue // Skip in-use interfaces
		}

		if err = netlink.LinkSetDown(tap); err != nil {
			return network, fmt.Errorf("could not bring %s link down: %v", tap.Name, err)
		}

		if err = netlink.LinkDel(tap); err != nil {
			return network, fmt.Errorf("could not remove %s: %v", tap.Name, err)
		}
	}

	return network, nil
}

// Delete implements kraftkit.sh/api/network/v1alpha1.Delete
func (service *v1alpha1Network) Delete(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	// Remove any interfaces.
	for _, iface := range network.Spec.Interfaces {
		// Get the link.
		link, err := netlink.LinkByName(iface.Spec.IfName)
		if err != nil {
			return network, fmt.Errorf("could not get %s link: %v", iface.Spec.IfName, err)
		}

		ip, _, err := net.ParseCIDR(iface.Spec.CIDR)
		if err != nil {
			return network, fmt.Errorf("could not parse IP address: %v", err)
		}

		if ping.Ping(&net.IPAddr{IP: ip, Zone: ""}, 150*time.Millisecond) {
			return network, fmt.Errorf("interface still in use: %s (%s, %s)", iface.Spec.IfName, iface.Spec.MacAddress, ip)
		}

		// Bring down the bridge link
		if err := netlink.LinkSetDown(link); err != nil {
			return network, fmt.Errorf("could not bring %s link down: %v", iface.Spec.IfName, err)
		}

		// Delete the bridge link.
		if err := netlink.LinkDel(link); err != nil {
			return network, fmt.Errorf("could not delete %s link: %v", iface.Spec.IfName, err)
		}
	}

	// Get the bridge link.
	link, err := netlink.LinkByName(network.Name)
	if err != nil {
		return network, fmt.Errorf("getting bridge %s failed: %v", network.Name, err)
	}

	// Bring down the bridge link
	if err := netlink.LinkSetDown(link); err != nil {
		return network, fmt.Errorf("could not bring %s link down: %v", network.Name, err)
	}

	// Delete the bridge link.
	if err := netlink.LinkDel(link); err != nil {
		return network, fmt.Errorf("could not delete %s link: %v", network.Name, err)
	}

	return nil, nil
}

// mapBridgeStatistics embeds the provided bridge's statistics to the provided
// network's status statistics, these are a 1-to-1 match.
func mapBridgeStatistics(network *networkv1alpha1.Network, bridge *netlink.Bridge) {
	network.Status.Collisions = bridge.Statistics.Collisions
	network.Status.Multicast = bridge.Statistics.Multicast
	network.Status.RxBytes = bridge.Statistics.RxBytes
	network.Status.RxCompressed = bridge.Statistics.RxCompressed
	network.Status.RxCrcErrors = bridge.Statistics.RxCrcErrors
	network.Status.RxDropped = bridge.Statistics.RxDropped
	network.Status.RxErrors = bridge.Statistics.RxErrors
	network.Status.RxFifoErrors = bridge.Statistics.RxFifoErrors
	network.Status.RxFrameErrors = bridge.Statistics.RxFrameErrors
	network.Status.RxLengthErrors = bridge.Statistics.RxLengthErrors
	network.Status.RxMissedErrors = bridge.Statistics.RxMissedErrors
	network.Status.RxOverErrors = bridge.Statistics.RxOverErrors
	network.Status.RxPackets = bridge.Statistics.RxPackets
	network.Status.TxAbortedErrors = bridge.Statistics.TxAbortedErrors
	network.Status.TxBytes = bridge.Statistics.TxBytes
	network.Status.TxCarrierErrors = bridge.Statistics.TxCarrierErrors
	network.Status.TxDropped = bridge.Statistics.TxDropped
	network.Status.TxErrors = bridge.Statistics.TxErrors
	network.Status.TxFifoErrors = bridge.Statistics.TxFifoErrors
	network.Status.TxHeartbeatErrors = bridge.Statistics.TxHeartbeatErrors
	network.Status.TxPackets = bridge.Statistics.TxPackets
	network.Status.TxWindowErrors = bridge.Statistics.TxWindowErrors
}

// Get implements kraftkit.sh/api/network/v1alpha1.Get
func (service *v1alpha1Network) Get(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	link, err := netlink.LinkByName(network.Name)
	if err != nil {
		return network, fmt.Errorf("could not get link %s: %v", network.Name, err)
	}

	if network.ObjectMeta.CreationTimestamp == *new(metav1.Time) {
		network.CreationTimestamp = metav1.Now()
	}

	bridge, ok := link.(*netlink.Bridge)
	if !ok {
		return network, fmt.Errorf("network link is not bridge")
	}

	addrs, err := netlink.AddrList(bridge, nl.FAMILY_V4)
	if err != nil {
		return network, err
	}

	if len(addrs) == 0 {
		return network, fmt.Errorf("bridge %s has no ip address", network.Name)
	}

	network.Spec.Driver = "bridge"
	network.Spec.Gateway = addrs[0].IP.String()
	network.Spec.Netmask = net.IP(addrs[0].Mask).String()

	// Use the internal network bridge networking system to determine
	// whether the identified network is online.
	if net.FlagUp&bridge.Flags == 1 || net.FlagRunning&bridge.Flags == 1 {
		network.Status.State = networkv1alpha1.NetworkStateUp
	} else {
		// TODO(nderjung): The bridge interface could be in other states.  For
		// now the v1alpha1.NetworkState does not support more complex states,
		// simply indicate that it is "down" and therefore unusable.
		network.Status.State = networkv1alpha1.NetworkStateDown
	}

	mapBridgeStatistics(network, bridge)

	return network, nil
}

// List implements kraftkit.sh/api/network/v1alpha1.List
func (service *v1alpha1Network) List(ctx context.Context, networks *networkv1alpha1.NetworkList) (*networkv1alpha1.NetworkList, error) {
	knownBridges := make(map[string]bool, len(networks.Items))

	// Update existing known networks
	for i, network := range networks.Items {
		network, err := service.Get(ctx, &network)
		if err != nil {
			continue // TODO(nderjung): error groups
		}

		networks.Items[i] = *network
		knownBridges[network.Spec.IfName] = true
	}

	handle, err := netlink.NewHandle()
	if err != nil {
		return nil, err
	}

	links, err := handle.LinkList()
	if err != nil {
		return nil, err
	}

	// Convert links to bridges and store in a map for fast access.
	bridges := map[string]*netlink.Bridge{}
	for _, link := range links {
		bridge, ok := link.(*netlink.Bridge)
		if !ok {
			continue // TODO(nderjung): error groups
		}

		if _, ok := knownBridges[bridge.Name]; ok {
			continue // Also skip known bridges
		}

		bridges[bridge.Name] = bridge
	}

	// Discover new bridges.
	for _, bridge := range bridges {
		addrs, err := netlink.AddrList(bridge, nl.FAMILY_V4)
		if err != nil {
			continue // TODO(nderjung): error groups
		}

		network := networkv1alpha1.Network{
			ObjectMeta: metav1.ObjectMeta{
				Name: bridge.Name,
				UID:  uuid.NewUUID(),
			},
		}

		if len(addrs) != 1 {
			network.Status.State = networkv1alpha1.NetworkStateDown
			networks.Items = append(networks.Items, network)
			continue // TODO(nderjung): error groups
		}

		network.Spec = networkv1alpha1.NetworkSpec{
			Gateway: addrs[0].IP.String(),
			Netmask: net.IP(addrs[0].Mask).String(),
		}

		// Use the internal network bridge networking system to determine
		// whether the identified network is online.
		if net.FlagUp&bridge.Flags == 1 || net.FlagRunning&bridge.Flags == 1 {
			network.Status.State = networkv1alpha1.NetworkStateUp
		} else {
			// TODO(nderjung): The bridge interface could be in other states.  For
			// now the v1alpha1.NetworkState does not support more complex states,
			// simply indicate that it is "down" and therefore unusable.
			network.Status.State = networkv1alpha1.NetworkStateDown
		}

		mapBridgeStatistics(&network, bridge)

		networks.Items = append(networks.Items, network)
	}

	// Sort networks by name before returning
	sort.SliceStable(networks.Items, func(i, j int) bool {
		iRunes := []rune(networks.Items[i].Name)
		jRunes := []rune(networks.Items[j].Name)

		max := len(iRunes)
		if max > len(jRunes) {
			max = len(jRunes)
		}

		for idx := 0; idx < max; idx++ {
			ir := iRunes[idx]
			jr := jRunes[idx]

			lir := unicode.ToLower(ir)
			ljr := unicode.ToLower(jr)

			if lir != ljr {
				return lir < ljr
			}

			// the lowercase runes are the same, so compare the original
			if ir != jr {
				return ir < jr
			}
		}

		// If the strings are the same up to the length of the shortest string,
		// the shorter string comes first
		return len(iRunes) < len(jRunes)
	})

	// TODO(nderjung): Return an error group.
	return networks, nil
}

// Watch implements kraftkit.sh/api/network/v1alpha1.Watch
func (service *v1alpha1Network) Watch(context.Context, *networkv1alpha1.Network) (chan *networkv1alpha1.Network, chan error, error) {
	panic("not implemented: kraftkit.sh/machine/network/bridge.v1alpha1Network.Watch")
}

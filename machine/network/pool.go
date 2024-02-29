// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package network

import (
	"fmt"
	"net"

	networkapi "kraftkit.sh/api/network/v1alpha1"
)

// NetworkPoolEntry describes a network to be used for allocating IP ranges.
type NetworkPoolEntry struct {
	// Subnet is the CIDR notation of the network to be sub-divided.
	Subnet string
	// Size is the size of the subnets to be allocated, in bits.
	Size int
}

type NetworkPool []NetworkPoolEntry

var DefaultNetworkPool = []NetworkPoolEntry{
	{"172.17.0.0/16", 16},
	{"172.18.0.0/16", 16},
	{"172.19.0.0/16", 16},
	{"172.20.0.0/14", 16},
	{"172.24.0.0/14", 16},
	{"172.28.0.0/14", 16},
	{"192.168.0.0/16", 20},
}

// FindFreeNetwork finds a free network in the pool.
func FindFreeNetwork(pool NetworkPool, existingNetworks *networkapi.NetworkList) (*net.IPNet, error) {
	convertedNetworks := []net.IPNet{}

	for _, network := range existingNetworks.Items {
		maskBytes := net.ParseIP(network.Spec.Netmask).To4()
		mask := net.IPv4Mask(maskBytes[0], maskBytes[1], maskBytes[2], maskBytes[3])
		// Setup IP address for bridge.
		convertedNetworks = append(convertedNetworks,
			net.IPNet{
				IP:   net.ParseIP(network.Spec.Gateway),
				Mask: mask,
			})
	}

	for _, poolEntry := range pool {
		startingIP, networkToSplit, err := net.ParseCIDR(poolEntry.Subnet)
		if err != nil {
			return nil, err
		}

		startingIP = startingIP.To4()

		// subnetIndex is an unsigned integer that is used to calculate the next
		var subnetIndex uint32 = 0

		ones, _ := networkToSplit.Mask.Size()
		numberOfSubnets := 1 << (uint(poolEntry.Size) - uint(ones))

		for range numberOfSubnets {
			candidateIp := make([]byte, len(startingIP))
			copy(candidateIp, startingIP)

			offset := subnetIndex
			for i := 3; i >= 0; i-- {
				candidateIp[i] |= byte(offset & 0xff)
				offset >>= 8
			}

			candidate := net.IPNet{
				IP:   (net.IP)(candidateIp).To4(),
				Mask: net.CIDRMask(poolEntry.Size, 32),
			}

			// Check if the candidate intersects with any existing network
			intersects := false
			for _, existing := range convertedNetworks {
				if NetworksIntersect(existing, candidate) {
					intersects = true
					break
				}
			}

			if !intersects {
				// Increment the candidate by 1 to get the first allocatable IP
				candidate.IP = candidate.IP.To4()
				candidate.IP[3] = candidate.IP[3] + 1

				return &candidate, nil
			}

			// Increment the candidate IP by the size of one subnetwork
			subnetIndex = subnetIndex + (1 << (32 - poolEntry.Size))
		}
	}

	return nil, fmt.Errorf("unable to find a free network in the network pool")
}

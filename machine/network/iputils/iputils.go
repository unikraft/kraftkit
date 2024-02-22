// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package iputils

import (
	"encoding/binary"
	"math/big"
	"net"
)

// IpToBigInt converts a 4 bytes IP into a 128 bit integer.
func IPToBigInt(ip net.IP) *big.Int {
	x := big.NewInt(0)
	if ip4 := ip.To4(); ip4 != nil {
		return x.SetBytes(ip4)
	}
	if ip6 := ip.To16(); ip6 != nil {
		return x.SetBytes(ip6)
	}
	return nil
}

// BigIntToIP converts 128 bit integer into a 4 bytes IP address.
func BigIntToIP(v *big.Int) net.IP {
	return net.IP(v.Bytes())
}

// Increases IP address numeric value by 1.
func IncreaseIP(ip net.IP) net.IP {
	rawip := IPToBigInt(ip)
	rawip.Add(rawip, big.NewInt(1))
	return BigIntToIP(rawip)
}

// IsUnicastIP returns true if the provided IP address and network mask is a
// unicast address.
func IsUnicastIP(ip net.IP, mask net.IPMask) bool {
	// broadcast v4 ip
	if len(ip) == net.IPv4len && binary.BigEndian.Uint32(ip)&^binary.BigEndian.Uint32(mask) == ^binary.BigEndian.Uint32(mask) {
		return false
	}

	// global unicast
	return ip.IsGlobalUnicast()
}

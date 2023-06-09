// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package macaddr

import (
	"crypto/rand"
	"net"
)

// GenerateMacAddress generates a pseudo-random MAC address.  A positional
// argument startAtZero can be set to true which results in the MAC address
// returning a pseudo-random MAC address with its last byte set to zero.  This
// is useful for iterative sequences of MAC addresses based on the same
// pseudo-random set.
func GenerateMacAddress(startAtZero bool) (net.HardwareAddr, error) {
	// TODO(nderjung): When startAtZero is set, we are generating one random byte
	// which is later discarded.
	buf := make([]byte, 3)
	var mac net.HardwareAddr
	if _, err := rand.Read(buf); err != nil {
		return mac, err
	}

	if startAtZero {
		buf[2] = 0
	}

	// 0x02 is a local address. 0xB0B0 is a Unikraft-specific identifier.
	mac = append(mac, 0x02, 0xB0, 0xB0, buf[0], buf[1], buf[2])

	return mac, nil
}

// IncrementMacAddress increases the provided MAC address by 1 such that the
// implementer can use this method to sequence a series of MAC addresses in a
// set.
func IncrementMacAddress(mac net.HardwareAddr) net.HardwareAddr {
	// Convert the last 3 bytes of the MAC address to an unsigned 32-bit integer.
	sum := uint32(mac[3])<<16 | uint32(mac[4])<<8 | uint32(mac[5])

	// Increment the integer representation of the last 3 bytes.
	sum++

	// Extract the individual bytes from the incremented integer.
	mac[3] = byte((sum >> 16) & 0xFF)
	mac[4] = byte((sum >> 8) & 0xFF)
	mac[5] = byte(sum & 0xFF)

	return mac
}

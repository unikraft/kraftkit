// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package network

import "net"

// NetworksIntersect returns whether two networks have any common IP addresses.
func NetworksIntersect(a, b net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package v1alpha1

import (
	"fmt"
	"strings"
)

// ParseNetwork parses a string representation of a MachineNetwork and returns
// the instantiated structure. The input format is:
//
//	name[:cidr[:gateway[:dns0[:dns1[:hostname[:domain]]]]]]
//
// Only the network name is required; all remaining fields are optional and
// parsed left-to-right.
func ParseNetwork(s string) (*MachineNetwork, error) {
	if s == "" {
		return nil, fmt.Errorf("network string cannot be empty")
	}

	parts := strings.SplitN(s, ":", 7)
	if parts[0] == "" {
		return nil, fmt.Errorf("network name cannot be empty")
	}

	network := &MachineNetwork{
		Name: parts[0],
	}

	if len(parts) > 1 {
		network.CIDR = parts[1]
	}
	if len(parts) > 2 {
		network.Gateway = parts[2]
	}
	if len(parts) > 3 {
		network.DNS0 = parts[3]
	}
	if len(parts) > 4 {
		network.DNS1 = parts[4]
	}
	if len(parts) > 5 {
		network.Hostname = parts[5]
	}
	if len(parts) > 6 {
		network.Domain = parts[6]
	}

	return network, nil
}

// String implements fmt.Stringer and outputs a MachineNetwork in the
// colon-separated format understood by ParseNetwork.
func (mn *MachineNetwork) String() string {
	fields := []string{mn.Name, mn.CIDR, mn.Gateway, mn.DNS0, mn.DNS1, mn.Hostname, mn.Domain}

	// Trim trailing empty fields.
	last := 0
	for i, f := range fields {
		if f != "" {
			last = i
		}
	}

	return strings.Join(fields[:last+1], ":")
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package uknetdev

import (
	"strings"

	"kraftkit.sh/unikraft/export/v0/ukargparse"
)

// The netdev.ip parameter can be used to override IPv4 address information
// for multiple devices. For each device the following colon-separated format
// is introduced:
//
// cidr[:gw[:dns0[:dns1[:hostname[:domain]]]]]
func NewParamIp() ukargparse.Param {
	return ukargparse.ParamStr("netdev", "ip", nil)
}

// NetdevIp represents the attributes of the network device which is understood
// by uknetdev and uklibparam.
type NetdevIp struct {
	CIDR     string
	Gateway  string
	DNS0     string
	DNS1     string
	Hostname string
	Domain   string
}

// String implements fmt.Stringer and returns a valid netdev.ip-formatted entry.
func (entry NetdevIp) String() string {
	return strings.Join([]string{
		entry.CIDR,
		entry.Gateway,
		entry.DNS0,
		entry.DNS1,
		entry.Hostname,
		entry.Domain,
	}, ":")
}

// ExportedParams returns the parameters available by this exported library.
func ExportedParams() []ukargparse.Param {
	return []ukargparse.Param{
		NewParamIp(),
	}
}

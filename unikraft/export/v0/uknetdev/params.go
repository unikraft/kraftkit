// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package uknetdev

import (
	"kraftkit.sh/unikraft/export/v0/ukargparse"
)

var (
	ParamIpv4Addr       = ukargparse.ParamStr("netdev", "ipv4_addr", nil)
	ParamIpv4SubnetMask = ukargparse.ParamStr("netdev", "ipv4_subnet_mask", nil)
	ParamIpv4GwAddr     = ukargparse.ParamStr("netdev", "ipv4_gw_addr", nil)
)

// ExportedParams returns the parameters available by this exported library.
func ExportedParams() []ukargparse.Param {
	return []ukargparse.Param{
		ParamIpv4Addr,
	}
}

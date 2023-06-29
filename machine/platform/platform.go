// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package platform

type Platform string

const (
	PlatformUnknown     = Platform("unknown")
	PlatformFirecracker = Platform("fc")
	PlatformQEMU        = Platform("qemu")
	PlatformKVM         = PlatformQEMU
	PlatformXen         = Platform("xen")
)

// String implements fmt.Stringer
func (ht Platform) String() string {
	return string(ht)
}

// Platforms returns the list of known platforms.
func Platforms() map[string]Platform {
	return map[string]Platform{
		"fc":   PlatformFirecracker,
		"kvm":  PlatformQEMU,
		"qemu": PlatformQEMU,
		"xen":  PlatformXen,
	}
}

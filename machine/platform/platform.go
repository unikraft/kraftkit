// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package platform

import "sort"

type Platform string

const (
	PlatformUnknown     = Platform("unknown")
	PlatformFirecracker = Platform("fc")
	PlatformQEMU        = Platform("qemu")
	PlatformKVM         = PlatformQEMU
	PlatformXen         = Platform("xen")
	PlatformHyperlight  = Platform("hyperlight")
)

// String implements fmt.Stringer
func (ht Platform) String() string {
	return string(ht)
}

// PlatformByName returns the platform for a given name.
// If the name is not known, it returns it unchanged.
func PlatformByName(name string) Platform {
	platforms := PlatformsByName()
	if _, ok := platforms[name]; !ok {
		return Platform(name)
	}
	return platforms[name]
}

// PlatformsByName returns the list of known platforms and their name alises.
func PlatformsByName() map[string]Platform {
	return map[string]Platform{
		"fc":          PlatformFirecracker,
		"firecracker": PlatformFirecracker,
		"kvm":         PlatformQEMU,
		"qemu":        PlatformQEMU,
		"xen":         PlatformXen,
		"hyperlight":  PlatformHyperlight,
		"hl":          PlatformHyperlight,
	}
}

// Platforms returns all the unique platforms.
func Platforms() []Platform {
	return []Platform{
		PlatformFirecracker,
		PlatformQEMU,
		PlatformXen,
		PlatformHyperlight,
	}
}

// PlatformNames returns all platform names and aliases suitable for user input.
func PlatformNames(extra ...Platform) []Platform {
	seen := map[Platform]struct{}{}
	ret := make([]Platform, 0, len(PlatformsByName())+len(extra))

	names := make([]string, 0, len(PlatformsByName()))
	for name := range PlatformsByName() {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		platform := Platform(name)
		if _, ok := seen[platform]; ok {
			continue
		}
		seen[platform] = struct{}{}
		ret = append(ret, platform)
	}

	for _, platform := range extra {
		if _, ok := seen[platform]; ok {
			continue
		}
		seen[platform] = struct{}{}
		ret = append(ret, platform)
	}

	return ret
}

// PlatformAliases returns all the name alises for a given platform.
func PlatformAliases() map[Platform][]string {
	aliases := map[Platform][]string{}

	for alias, plat := range PlatformsByName() {
		if aliases[plat] == nil {
			aliases[plat] = make([]string, 0)
		}

		aliases[plat] = append(aliases[plat], alias)
	}

	return aliases
}

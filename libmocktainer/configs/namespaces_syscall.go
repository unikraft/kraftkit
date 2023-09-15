//go:build linux
// +build linux

// SPDX-License-Identifier: Apache-2.0
// Copyright 2014 Docker, Inc.
// Copyright 2023 Unikraft GmbH and The KraftKit Authors

package configs

import "golang.org/x/sys/unix"

var namespaceInfo = map[NamespaceType]int{
	NEWNET: unix.CLONE_NEWNET,
}

// CloneFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare. This function returns flags only for new namespaces.
func (n *Namespaces) CloneFlags() uintptr {
	var flag int
	for _, v := range *n {
		if v.Path != "" {
			continue
		}
		flag |= namespaceInfo[v.Type]
	}
	return uintptr(flag)
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/containerd/nerdctl/v2/pkg/portutil"

	corev1 "k8s.io/api/core/v1"
)

const DefaultProtocol = corev1.ProtocolTCP

// ParsePort parses a string representation of a MachinePort and returns the
// instantiated structure.  The input structure uses nerdctl's implementation
// which in itself follows the traditional "docker-like" syntax often used in a
// CLI-context with the `-p` flag.
func ParsePort(s string) ([]MachinePort, error) {
	mappings, err := portutil.ParseFlagP(s)
	if err != nil {
		return nil, err
	}

	ports := make([]MachinePort, len(mappings))
	for i, port := range mappings {
		ports[i] = MachinePort{
			HostIP:      port.HostIP,
			HostPort:    port.HostPort,
			MachinePort: port.ContainerPort,
			Protocol:    corev1.Protocol(port.Protocol),
		}
	}

	return ports, nil
}

// String implements fmt.Stringer and outputs MachinePorts in human-readable
// format.
func (ports MachinePorts) String() string {
	strs := make([]string, len(ports))

	for i, p := range ports {
		strs[i] = fmt.Sprintf("%s:%d->%d/%s", p.HostIP, p.HostPort, p.MachinePort, p.Protocol)
	}

	return strings.Join(strs, ", ")
}

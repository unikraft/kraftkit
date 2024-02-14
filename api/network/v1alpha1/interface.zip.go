// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package v1alpha1

import (
	zip "api.zip"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	// NetworkInterface is the mutable API object that represents network
	// interfaces.
	NetworkInterface = zip.Object[NetworkInterfaceSpec, NetworkInterfaceStatus]

	// NetworkInterfaceList is the mutable API object that represents a list of
	// networks.
	NetworkInterfaceList = zip.ObjectList[NetworkInterfaceSpec, NetworkInterfaceStatus]
)

// NetworkInterfaceSpec represents a specific network interface which is
// situated on the network.
type NetworkInterfaceSpec struct {
	// The name of the interface.
	IfName string `json:"ifname,omitempty"`

	// IPv4 address in CIDR notation, which includes the subnet.
	CIDR string

	// Gateway IPv4 address.
	Gateway string

	// IPv4 address of the primary DNS server.
	DNS0 string

	// IPv4 address of the secondary DNS server.
	DNS1 string

	// Hostname of the IPv4 address.
	Hostname string

	// Domain/Search suffix for IPv4 address.
	Domain string

	// Hardware address of a machine interface.
	MacAddress string `json:"mac,omitempty"`
}

// NetworkInterfaceTemplateSpec describes the data a network interface should
// have when created from a template.
type NetworkInterfaceTemplateSpec struct {
	// Metadata of the pods created from this template.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of the network interface.
	Spec NetworkInterfaceSpec `json:"spec,omitempty"`
}

// NetworkInterfaceState indicates the state of the network.
type NetworkInterfaceState string

const (
	NetworkInterfaceStateUnknown      = NetworkInterfaceState("unknown")
	NetworkInterfaceStateConnected    = NetworkInterfaceState("connected")
	NetworkInterfaceStateDisconnected = NetworkInterfaceState("disconnected")
)

// String implements fmt.Stringer
func (ms NetworkInterfaceState) String() string {
	return string(ms)
}

type NetworkInterfaceStatus struct {
	// State is the current state of the network interface.
	State NetworkInterfaceState `json:"state"`
}

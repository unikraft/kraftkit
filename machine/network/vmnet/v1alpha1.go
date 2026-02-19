// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package vmnet

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	networkv1alpha1 "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/log"
	"kraftkit.sh/machine/network/macaddr"
)

const (
	// DefaultVmnetMode is the default vmnet mode for macOS (shared provides NAT)
	DefaultVmnetMode = "shared"
	// DefaultGateway is the default gateway for vmnet-shared mode
	DefaultGateway = "192.168.64.1"
	// DefaultNetmask is the default netmask for vmnet-shared mode  
	DefaultNetmask = "255.255.255.0"
)

type v1alpha1Network struct{}

func NewNetworkServiceV1alpha1(ctx context.Context, opts ...any) (networkv1alpha1.NetworkService, error) {
	return &v1alpha1Network{}, nil
}

// Create implements kraftkit.sh/api/network/v1alpha1.Create
func (service *v1alpha1Network) Create(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	if network.Name == "" {
		return nil, fmt.Errorf("cannot create network without name")
	}

	if network.ObjectMeta.UID != "" {
		return network, fmt.Errorf("network already exists: %s", network.Name)
	}

	network.ObjectMeta.UID = uuid.NewUUID()

	if network.Spec.IfName == "" {
		network.Spec.IfName = network.Name
	}

	network.Spec.Driver = "vmnet-" + DefaultVmnetMode
	network.Status.State = networkv1alpha1.NetworkStateUnknown

	// Set default gateway and netmask for macOS vmnet-shared mode
	if len(network.Spec.Gateway) == 0 {
		network.Spec.Gateway = DefaultGateway
	}
	if len(network.Spec.Netmask) == 0 {
		network.Spec.Netmask = DefaultNetmask
	}

	// Validate the gateway and netmask
	if net.ParseIP(network.Spec.Gateway) == nil {
		return nil, fmt.Errorf("invalid gateway IP address: %s", network.Spec.Gateway)
	}
	if net.ParseIP(network.Spec.Netmask) == nil {
		return nil, fmt.Errorf("invalid netmask: %s", network.Spec.Netmask)
	}

	// On macOS, vmnet networks are managed by the OS/QEMU, so we just track metadata
	network.CreationTimestamp = metav1.Now()
	network.Status.State = networkv1alpha1.NetworkStateDown

	log.G(ctx).Infof("Created vmnet-shared network %s (gateway: %s, netmask: %s)", 
		network.Name, network.Spec.Gateway, network.Spec.Netmask)

	return network, nil
}

// Start implements kraftkit.sh/api/network/v1alpha1.Start
func (service *v1alpha1Network) Start(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	if network.UID == "" {
		return nil, fmt.Errorf("cannot start network without UID")
	}

	// On macOS, vmnet networks are activated when VMs are started
	// We just update the state to indicate it's ready
	network.Status.State = networkv1alpha1.NetworkStateUp
	
	log.G(ctx).Infof("Started vmnet network %s", network.Name)
	
	return network, nil
}

// Stop implements kraftkit.sh/api/network/v1alpha1.Stop
func (service *v1alpha1Network) Stop(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	if network.UID == "" {
		return nil, fmt.Errorf("cannot stop network without UID")
	}

	// On macOS, vmnet networks are deactivated when VMs are stopped
	network.Status.State = networkv1alpha1.NetworkStateDown
	
	log.G(ctx).Infof("Stopped vmnet network %s", network.Name)
	
	return network, nil
}

// Update implements kraftkit.sh/api/network/v1alpha1.Update
func (service *v1alpha1Network) Update(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	if network.UID == "" {
		return nil, fmt.Errorf("cannot update network without UID")
	}

	// Parse the network configuration
	_, ipnet, err := net.ParseCIDR(fmt.Sprintf("%s/%s", network.Spec.Gateway, network.Spec.Netmask))
	if err != nil {
		mask := net.IPMask(net.ParseIP(network.Spec.Netmask).To4())
		ones, _ := mask.Size()
		ipnet = &net.IPNet{
			IP:   net.ParseIP(network.Spec.Gateway),
			Mask: mask,
		}
		if ones == 0 {
			return network, fmt.Errorf("invalid netmask: %s", network.Spec.Netmask)
		}
	}

	// Start MAC addresses iteratively
	startMac, err := macaddr.GenerateMacAddress(true)
	if err != nil {
		return network, fmt.Errorf("could not prepare MAC address generator: %v", err)
	}

	// Add any defined interfaces
	for i, iface := range network.Spec.Interfaces {
		if iface.ObjectMeta.UID == "" {
			iface.ObjectMeta.UID = uuid.NewUUID()
		}

		if iface.Spec.IfName == "" {
			iface.Spec.IfName = fmt.Sprintf("%s-if%d", network.Name, i)
		}

		if iface.Spec.MacAddress == "" {
			startMac = macaddr.IncrementMacAddress(startMac)
			iface.Spec.MacAddress = startMac.String()
		}

		// Set interface gateway if not specified
		if iface.Spec.Gateway == "" {
			iface.Spec.Gateway = network.Spec.Gateway
		}

		// Generate CIDR if not specified
		if iface.Spec.CIDR == "" {
			// Assign an IP from the subnet
			ip := make(net.IP, len(ipnet.IP))
			copy(ip, ipnet.IP)
			ip[len(ip)-1] += byte(i + 2) // Start from .2, .3, etc.
			
			ones, _ := ipnet.Mask.Size()
			iface.Spec.CIDR = fmt.Sprintf("%s/%d", ip.String(), ones)
		}

		network.Spec.Interfaces[i] = iface
	}

	log.G(ctx).Debugf("Updated vmnet network %s with %d interfaces", network.Name, len(network.Spec.Interfaces))

	return network, nil
}

// Delete implements kraftkit.sh/api/network/v1alpha1.Delete
func (service *v1alpha1Network) Delete(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	// On macOS, vmnet networks are managed by the OS, so we just clean up metadata
	log.G(ctx).Infof("Deleted vmnet network %s", network.Name)
	
	return nil, nil
}

// Get implements kraftkit.sh/api/network/v1alpha1.Get
func (service *v1alpha1Network) Get(ctx context.Context, network *networkv1alpha1.Network) (*networkv1alpha1.Network, error) {
	if network.UID == "" {
		return nil, fmt.Errorf("no such network: %s", network.Name)
	}

	// Check if network interface exists via ifconfig
	// This is a basic check - on macOS, vmnet interfaces are ephemeral
	cmd := exec.CommandContext(ctx, "ifconfig", network.Spec.IfName)
	output, err := cmd.CombinedOutput()
	
	if err != nil || len(output) == 0 {
		// Interface doesn't exist or is not active
		network.Status.State = networkv1alpha1.NetworkStateDown
	} else {
		// Interface exists and is active
		network.Status.State = networkv1alpha1.NetworkStateUp
		
		// Try to parse statistics from ifconfig output
		service.parseIfconfigStats(network, string(output))
	}

	network.Spec.Driver = "vmnet-" + DefaultVmnetMode

	return network, nil
}

// List implements kraftkit.sh/api/network/v1alpha1.List
func (service *v1alpha1Network) List(ctx context.Context, networks *networkv1alpha1.NetworkList) (*networkv1alpha1.NetworkList, error) {
	// Update existing known networks
	validNetworks := make([]networkv1alpha1.Network, 0, len(networks.Items))
	
	for _, network := range networks.Items {
		network, err := service.Get(ctx, &network)
		if err != nil {
			// Skip networks that no longer exist
			log.G(ctx).Debugf("skipping network %s: %v", network.Name, err)
			continue
		}
		
		validNetworks = append(validNetworks, *network)
	}
	
	networks.Items = validNetworks

	return networks, nil
}

// parseIfconfigStats parses statistics from ifconfig output
func (service *v1alpha1Network) parseIfconfigStats(network *networkv1alpha1.Network, output string) {
	lines := strings.Split(output, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Look for input/output packet stats
		// Example: "input: 1234 packets, 567890 bytes"
		if strings.HasPrefix(line, "input:") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "packets" && i > 0 {
					fmt.Sscanf(fields[i-1], "%d", &network.Status.RxPackets)
				}
				if field == "bytes" && i > 0 {
					fmt.Sscanf(fields[i-1], "%d", &network.Status.RxBytes)
				}
			}
		}
		
		if strings.HasPrefix(line, "output:") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "packets" && i > 0 {
					fmt.Sscanf(fields[i-1], "%d", &network.Status.TxPackets)
				}
				if field == "bytes" && i > 0 {
					fmt.Sscanf(fields[i-1], "%d", &network.Status.TxBytes)
				}
			}
		}
	}
}

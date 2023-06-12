// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package v1alpha1

import (
	"context"

	zip "api.zip"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	// Network is the mutable API object that represents a network.
	Network = zip.Object[NetworkSpec, NetworkStatus]

	// NetworkList is the mutable API object that represents a list of networks.
	NetworkList = zip.ObjectList[NetworkSpec, NetworkStatus]
)

// NetworkSpec contains the desired behavior of the network.
type NetworkSpec struct {
	// Driver is the name of the implementing strategy.
	Driver string `json:"driver,omitempty"`

	// Interface name of this network.
	IfName string `json:"ifName,omitempty"`

	// The gateway IP address of the network.
	Gateway string `json:"gateway,omitempty"`

	// The network mask to apply over the gateway IP address to gather the subnet
	// range.
	Netmask string `json:"netmask,omitempty"`

	// Network interfaces associated with this network.
	Interfaces []NetworkInterfaceTemplateSpec `json:"interfaces,omitempty"`
}

// NetworkTemplateSpec describes the data a network should have when created
// from a template.
type NetworkTemplateSpec struct {
	// Metadata of the pods created from this template.
	metav1.ObjectMeta

	// Spec defines the behavior of the network.
	Spec NetworkSpec
}

// NetworkState indicates the state of the network.
type NetworkState string

const (
	NetworkStateUnknown = NetworkState("unknown")
	NetworkStateUp      = NetworkState("up")
	NetworkStateDown    = NetworkState("down")
)

// String implements fmt.Stringer
func (ms NetworkState) String() string {
	return string(ms)
}

// NetworkStatus contains the complete status of the network.
type NetworkStatus struct {
	// State is the current state of the network.
	State NetworkState `json:"state"`

	// Statistics
	Collisions        uint64 `json:"collisions"`
	Multicast         uint64 `json:"multicast"`
	RxBytes           uint64 `json:"rxBytes"`
	RxCompressed      uint64 `json:"rxCompressed"`
	RxCrcErrors       uint64 `json:"rxCrcErrors"`
	RxDropped         uint64 `json:"rxDropped"`
	RxErrors          uint64 `json:"rxErrors"`
	RxFifoErrors      uint64 `json:"rxFifoErrors"`
	RxFrameErrors     uint64 `json:"rxFrameErrors"`
	RxLengthErrors    uint64 `json:"rxLengthErrors"`
	RxMissedErrors    uint64 `json:"rxMissedErrors"`
	RxOverErrors      uint64 `json:"rxOverErrors"`
	RxPackets         uint64 `json:"rxPackets"`
	TxAbortedErrors   uint64 `json:"txAbortedErrors"`
	TxBytes           uint64 `json:"txBytes"`
	TxCarrierErrors   uint64 `json:"txCarrierErrors"`
	TxCompressed      uint64 `json:"txCompressed"`
	TxDropped         uint64 `json:"txDropped"`
	TxErrors          uint64 `json:"txErrors"`
	TxFifoErrors      uint64 `json:"txFifoErrors"`
	TxHeartbeatErrors uint64 `json:"txHeartbeatErrors"`
	TxPackets         uint64 `json:"txPackets"`
	TxWindowErrors    uint64 `json:"txWindowErrors"`

	// DriverConfig is driver-specific attributes which are populated by the
	// underlying network implementation.
	DriverConfig interface{} `json:"driverConfig,omitempty"`
}

// NetworkService is the interface of available methods which can be performed
// by an implementing network driver.
type NetworkService interface {
	Create(context.Context, *Network) (*Network, error)
	Start(context.Context, *Network) (*Network, error)
	Stop(context.Context, *Network) (*Network, error)
	Update(context.Context, *Network) (*Network, error)
	Delete(context.Context, *Network) (*Network, error)
	Get(context.Context, *Network) (*Network, error)
	List(context.Context, *NetworkList) (*NetworkList, error)
}

// NetworkServiceHandler provides a Zip API Object Framework service for the
// network.
type NetworkServiceHandler struct {
	create zip.MethodStrategy[*Network, *Network]
	start  zip.MethodStrategy[*Network, *Network]
	stop   zip.MethodStrategy[*Network, *Network]
	update zip.MethodStrategy[*Network, *Network]
	delete zip.MethodStrategy[*Network, *Network]
	get    zip.MethodStrategy[*Network, *Network]
	list   zip.MethodStrategy[*NetworkList, *NetworkList]
}

// Create implements NetworkService
func (client *NetworkServiceHandler) Create(ctx context.Context, req *Network) (*Network, error) {
	return client.create.Do(ctx, req)
}

// Start implements NetworkService
func (client *NetworkServiceHandler) Start(ctx context.Context, req *Network) (*Network, error) {
	return client.start.Do(ctx, req)
}

// Stop implements NetworkService
func (client *NetworkServiceHandler) Stop(ctx context.Context, req *Network) (*Network, error) {
	return client.stop.Do(ctx, req)
}

// Update implements NetworkService
func (client *NetworkServiceHandler) Update(ctx context.Context, req *Network) (*Network, error) {
	return client.update.Do(ctx, req)
}

// Delete implements NetworkService
func (client *NetworkServiceHandler) Delete(ctx context.Context, req *Network) (*Network, error) {
	return client.delete.Do(ctx, req)
}

// Get implements NetworkService
func (client *NetworkServiceHandler) Get(ctx context.Context, req *Network) (*Network, error) {
	return client.get.Do(ctx, req)
}

// List implements NetworkService
func (client *NetworkServiceHandler) List(ctx context.Context, req *NetworkList) (*NetworkList, error) {
	return client.list.Do(ctx, req)
}

// NewNetworkServiceHandler returns a service based on an inline API
// client which essentially wraps the specific call, enabling pre- and post-
// call hooks.  This is useful for wrapping the command with decorators, for
// example, a cache, error handlers, etc.  Simultaneously, it enables access to
// the service via inline code without having to make invocations to an external
// handler.
func NewNetworkServiceHandler(ctx context.Context, impl NetworkService, opts ...zip.ClientOption) (NetworkService, error) {
	create, err := zip.NewMethodClient(ctx, impl.Create, opts...)
	if err != nil {
		return nil, err
	}

	start, err := zip.NewMethodClient(ctx, impl.Start, opts...)
	if err != nil {
		return nil, err
	}

	stop, err := zip.NewMethodClient(ctx, impl.Stop, opts...)
	if err != nil {
		return nil, err
	}

	update, err := zip.NewMethodClient(ctx, impl.Update, opts...)
	if err != nil {
		return nil, err
	}

	delete, err := zip.NewMethodClient(ctx, impl.Delete, opts...)
	if err != nil {
		return nil, err
	}

	get, err := zip.NewMethodClient(ctx, impl.Get, opts...)
	if err != nil {
		return nil, err
	}

	list, err := zip.NewMethodClient(ctx, impl.List, opts...)
	if err != nil {
		return nil, err
	}

	return &NetworkServiceHandler{
		create,
		start,
		stop,
		update,
		delete,
		get,
		list,
	}, nil
}

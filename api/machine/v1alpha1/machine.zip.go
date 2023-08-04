// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package v1alpha1

import (
	"context"
	"time"

	zip "api.zip"
	corev1 "k8s.io/api/core/v1"

	networkv1alpha1 "kraftkit.sh/api/network/v1alpha1"
	volumev1alpha1 "kraftkit.sh/api/volume/v1alpha1"
)

// MachinePort represents a network port in a single container.
type MachinePort struct {
	// If specified, this must be an IANA_SVC_NAME and unique within the host.
	// Each named port on the host must have a unique name. Name for the port that
	// can be referred to by services.
	Name string `json:"name,omitempty"`

	// Number of port to expose on the host.  If specified, this must be a valid
	// port number, 0 < x < 65536.
	HostPort int32 `json:"hostPort,omitempty"`

	// Number of port to expose on the machine's IP address. This must be a valid
	// port number, 0 < x < 65536.
	MachinePort int32 `json:"machinePort"`

	// Protocol for port. Must be UDP or TCP. Defaults to "TCP".
	Protocol corev1.Protocol `json:"protocol,omitempty"`

	// What host IP to bind the external port to.
	HostIP string `json:"hostIP,omitempty"`

	// MAC address of the port.
	MacAddress string `json:"macAddress,omitempty"`
}

// MachinePorts is a slice of MachinePort
type MachinePorts []MachinePort

type (
	// Machine is the mutable API object that represents a machine instance.
	Machine = zip.Object[MachineSpec, MachineStatus]

	// MachineList is the mutable API object that represents a list of machine
	// instances.
	MachineList = zip.ObjectList[MachineSpec, MachineStatus]
)

// MachineSpec contains the desired behavior of the Machine.
type MachineSpec struct {
	// Architecture of the machine instance.
	Architecture string `json:"arch,omitempty"`

	// Platform of the machine instance.
	Platform string `json:"plat,omitempty"`

	// Kernel represents the
	Kernel string `json:"kernel,omitempty"`

	// Rootfs the fully-qualified path to the target root file system.  This can
	// be device path, a mount-path, initramdisk.
	Rootfs string `json:"rootfs,omitempty"`

	// Kernel arguments are runtime arguments which are provided directly to the
	// kernel and not for the application.
	KernelArgs []string `json:"kernelArgs,omitempty"`

	// Application arguments are runtime arguments provided to the application and
	// not the kernel.
	ApplicationArgs []string `json:"args,omitempty"`

	// Ports lists the ports and their mappings
	Ports MachinePorts `json:"ports,omitempty"`

	// Networks associated with this machine.
	Networks []networkv1alpha1.NetworkSpec `json:"networks,omitempty"`

	// Volumes associated with this machine.
	Volumes []volumev1alpha1.Volume `json:"volumes,omitempty"`

	// Resources describes the compute resources (requests and limits) required by
	// this machine.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Emulation indicates whether to use VMM emulation.
	Emulation bool `json:"emulation,omitempty"`
}

// MachineState indicates the state of the machine.
type MachineState string

const (
	MachineStateUnknown    = MachineState("unknown")
	MachineStateCreated    = MachineState("created")
	MachineStateFailed     = MachineState("failed")
	MachineStateRestarting = MachineState("restarting")
	MachineStateRunning    = MachineState("running")
	MachineStatePaused     = MachineState("paused")
	MachineStateSuspended  = MachineState("suspended")
	MachineStateExited     = MachineState("exited")
	MachineStateErrored    = MachineState("errored")
)

// String implements fmt.Stringer
func (ms MachineState) String() string {
	return string(ms)
}

// MachineStatus contains the complete status of the machine instance.
type MachineStatus struct {
	// State is the current state of the machine instance.
	State MachineState `json:"state"`

	// Pid of the machine instance (if applicable).
	Pid int32 `json:"pid,omitempty"`

	// The fully-qualified path to the kernel image of the machine instance.
	KernelPath string `json:"kernelPath,omitempty"`

	// The fully-qualified path to the initramfs file of the machine instance.
	InitrdPath string `json:"initrdPath,omitempty"`

	// ExitCode is the ...
	ExitCode int `json:"exitCode,omitempty"`

	// StartedAt represents when the machine was started.
	StartedAt time.Time `json:"startedAt,omitempty"`

	// ExitedAt represents when the machine fully shutdown
	ExitedAt time.Time `json:"exitedAt,omitempty"`

	// StateDir contains the path of the state of the machine.
	StateDir string `json:"stateDir,omitempty"`

	// LogFile is the in-host path to the log file of the machine.
	LogFile string `json:"logFile,omitempty"`

	// PlatformConfig is platform-specific attributes which are populated by the
	// underlying machine service implementation.
	PlatformConfig interface{} `json:"platformConfig,omitempty"`
}

// MachineService is the interface of available methods which can be performed
// by an implementing machine platform driver.
type MachineService interface {
	Create(context.Context, *Machine) (*Machine, error)
	Start(context.Context, *Machine) (*Machine, error)
	Pause(context.Context, *Machine) (*Machine, error)
	Stop(context.Context, *Machine) (*Machine, error)
	Update(context.Context, *Machine) (*Machine, error)
	Delete(context.Context, *Machine) (*Machine, error)
	Get(context.Context, *Machine) (*Machine, error)
	List(context.Context, *MachineList) (*MachineList, error)
	Watch(context.Context, *Machine) (chan *Machine, chan error, error)
	Logs(context.Context, *Machine) (chan string, chan error, error)
}

// MachineServiceHandler provides a Zip API Object Framework service for the
// machine.
type MachineServiceHandler struct {
	create zip.MethodStrategy[*Machine, *Machine]
	start  zip.MethodStrategy[*Machine, *Machine]
	pause  zip.MethodStrategy[*Machine, *Machine]
	stop   zip.MethodStrategy[*Machine, *Machine]
	update zip.MethodStrategy[*Machine, *Machine]
	delete zip.MethodStrategy[*Machine, *Machine]
	get    zip.MethodStrategy[*Machine, *Machine]
	list   zip.MethodStrategy[*MachineList, *MachineList]
	watch  zip.StreamStrategy[*Machine, *Machine]
	logs   zip.StreamStrategy[*Machine, string]
}

// Create implements MachineService
func (client *MachineServiceHandler) Create(ctx context.Context, req *Machine) (*Machine, error) {
	return client.create.Do(ctx, req)
}

// Start implements MachineService
func (client *MachineServiceHandler) Start(ctx context.Context, req *Machine) (*Machine, error) {
	return client.start.Do(ctx, req)
}

// Pause implements MachineService
func (client *MachineServiceHandler) Pause(ctx context.Context, req *Machine) (*Machine, error) {
	return client.pause.Do(ctx, req)
}

// Stop implements MachineService
func (client *MachineServiceHandler) Stop(ctx context.Context, req *Machine) (*Machine, error) {
	return client.stop.Do(ctx, req)
}

// Update implements MachineService
func (client *MachineServiceHandler) Update(ctx context.Context, req *Machine) (*Machine, error) {
	return client.update.Do(ctx, req)
}

// Delete implements MachineService
func (client *MachineServiceHandler) Delete(ctx context.Context, req *Machine) (*Machine, error) {
	return client.delete.Do(ctx, req)
}

// Get implements MachineService
func (client *MachineServiceHandler) Get(ctx context.Context, req *Machine) (*Machine, error) {
	return client.get.Do(ctx, req)
}

// List implements MachineService
func (client *MachineServiceHandler) List(ctx context.Context, req *MachineList) (*MachineList, error) {
	return client.list.Do(ctx, req)
}

// Watch implements MachineService
func (client *MachineServiceHandler) Watch(ctx context.Context, req *Machine) (chan *Machine, chan error, error) {
	return client.watch.Channel(ctx, req)
}

// Logs implements MachineService
func (client *MachineServiceHandler) Logs(ctx context.Context, req *Machine) (chan string, chan error, error) {
	return client.logs.Channel(ctx, req)
}

// NewMachineServiceHandler returns a service based on an inline API
// client which essentially wraps the specific call, enabling pre- and post-
// call hooks.  This is useful for wrapping the command with decorators, for
// example, a cache, error handlers, etc.  Simultaneously, it enables access to
// the service via inline code without having to make invocations to an external
// handler.
func NewMachineServiceHandler(ctx context.Context, impl MachineService, opts ...zip.ClientOption) (MachineService, error) {
	create, err := zip.NewMethodClient(ctx, impl.Create, opts...)
	if err != nil {
		return nil, err
	}

	start, err := zip.NewMethodClient(ctx, impl.Start, opts...)
	if err != nil {
		return nil, err
	}

	pause, err := zip.NewMethodClient(ctx, impl.Pause, opts...)
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

	watch, err := zip.NewStreamClient(ctx, impl.Watch, opts...)
	if err != nil {
		return nil, err
	}

	logs, err := zip.NewStreamClient(ctx, impl.Logs, opts...)
	if err != nil {
		return nil, err
	}

	return &MachineServiceHandler{
		create,
		start,
		pause,
		stop,
		update,
		delete,
		get,
		list,
		watch,
		logs,
	}, nil
}

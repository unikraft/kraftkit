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
	// Volume is the mutable API object that represents a volume.
	Volume = zip.Object[VolumeSpec, VolumeStatus]

	// VolumeList is the mutable API object that represents a list of volumes.
	VolumeList = zip.ObjectList[VolumeSpec, VolumeStatus]
)

// VolumeSpec contains the desired behavior of the volume.
type VolumeSpec struct {
	// Driver is the name of the implementing strategy.  Volume drivers let you
	// store volumes on remote hosts or cloud providers, to encrypt the contents
	// of volumes, or to add other functionality.
	Driver string `json:"driver,omitempty"`

	// The source of the mount.  For named volumes, this is the name of the
	// volume.  For anonymous volumes, this field is omitted.
	Source string `json:"source,omitempty"`

	// The destination takes as its value the path where the file or directory is
	// mounted in the machine.
	Destination string `json:"destination,omitempty"`

	// File permission mode (Linux only).
	Mode string `json:"mode,omitempty"`

	// Mark whether the volume is readonly.
	ReadOnly bool `json:"readOnly,omitempty"`

	// Managed is a flag that indicates whether the volume is managed
	// by kraftkit or not.
	Managed bool `json:"managed,omitempty"`
}

// VolumeTemplateSpec describes the data a volume should have when created
// from a template.
type VolumeTemplateSpec struct {
	// Metadata of the components used or created from this template.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the behavior of the volume.
	Spec VolumeSpec `json:"spec,omitempty"`
}

// VolumeState indicates the state of the volume.
type VolumeState string

const (
	// used for PersistentVolumeClaims that are not yet bound
	VolumeStatePending = VolumeState("Pending")
	// used for PersistentVolumeClaims that are bound
	VolumeStateBound = VolumeState("Bound")
	// used for PersistentVolumeClaims that lost their underlying
	// PersistentVolume. The claim was bound to a PersistentVolume and this
	// volume does not exist any longer and all data on it was lost.
	VolumeStateLost = VolumeState("Lost")
)

// String implements fmt.Stringer
func (vs VolumeState) String() string {
	return string(vs)
}

// VolumeStatus contains the complete status of the volume.
type VolumeStatus struct {
	// State is the current state of the volume.
	State VolumeState `json:"state"`

	// DriverConfig is driver-specific attributes which are populated by the
	// underlying volume implementation.
	DriverConfig interface{} `json:"driverConfig,omitempty"`
}

// VolumeService is the interface of available methods which can be performed
// by an implementing network driver.
type VolumeService interface {
	Create(context.Context, *Volume) (*Volume, error)
	Delete(context.Context, *Volume) (*Volume, error)
	Get(context.Context, *Volume) (*Volume, error)
	List(context.Context, *VolumeList) (*VolumeList, error)
	Update(context.Context, *Volume) (*Volume, error)
}

// VolumeServiceHandler provides a Zip API Object Framework service for the
// volume.
type VolumeServiceHandler struct {
	create zip.MethodStrategy[*Volume, *Volume]
	delete zip.MethodStrategy[*Volume, *Volume]
	get    zip.MethodStrategy[*Volume, *Volume]
	list   zip.MethodStrategy[*VolumeList, *VolumeList]
	update zip.MethodStrategy[*Volume, *Volume]
}

// Create implements VolumeService
func (client *VolumeServiceHandler) Create(ctx context.Context, req *Volume) (*Volume, error) {
	return client.create.Do(ctx, req)
}

// Delete implements VolumeService
func (client *VolumeServiceHandler) Delete(ctx context.Context, req *Volume) (*Volume, error) {
	return client.delete.Do(ctx, req)
}

// Get implements VolumeService
func (client *VolumeServiceHandler) Get(ctx context.Context, req *Volume) (*Volume, error) {
	return client.get.Do(ctx, req)
}

// List implements VolumeService
func (client *VolumeServiceHandler) List(ctx context.Context, req *VolumeList) (*VolumeList, error) {
	return client.list.Do(ctx, req)
}

// Update implements VolumeService
func (client *VolumeServiceHandler) Update(ctx context.Context, req *Volume) (*Volume, error) {
	return client.update.Do(ctx, req)
}

// NewVolumeServiceHandler returns a service based on an inline API
// client which essentially wraps the specific call, enabling pre- and post-
// call hooks.  This is useful for wrapping the command with decorators, for
// example, a cache, error handlers, etc.  Simultaneously, it enables access to
// the service via inline code without having to make invocations to an external
// handler.
func NewVolumeServiceHandler(ctx context.Context, impl VolumeService, opts ...zip.ClientOption) (VolumeService, error) {
	create, err := zip.NewMethodClient(ctx, impl.Create, opts...)
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

	update, err := zip.NewMethodClient(ctx, impl.Update, opts...)
	if err != nil {
		return nil, err
	}

	return &VolumeServiceHandler{
		create,
		delete,
		get,
		list,
		update,
	}, nil
}

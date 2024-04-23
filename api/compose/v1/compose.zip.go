// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package v1

import (
	"context"

	zip "api.zip"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	// Compose is the mutable API object that represents a compose project.
	Compose = zip.Object[ComposeSpec, ComposeStatus]

	// ComposeList is the mutable API object that represents a list of compose
	// projects.
	ComposeList = zip.ObjectList[ComposeSpec, ComposeStatus]
)

// ComposeSpec uniquely identifies a compose project.
type ComposeSpec struct {
	Workdir     string `json:"workdir,omitempty"`
	Composefile string `json:"composefile,omitempty"`
}

// ComposeStatus contains the complete status of the compose project.
type ComposeStatus struct {
	Machines []v1.ObjectMeta `json:"machines,omitempty"`
	Networks []v1.ObjectMeta `json:"networks,omitempty"`
	Volumes  []v1.ObjectMeta `json:"volumes,omitempty"`
}

// ComposeService is the interface of available methods
type ComposeService interface {
	Create(ctx context.Context, req *Compose) (*Compose, error)
	Delete(ctx context.Context, req *Compose) (*Compose, error)
	Get(ctx context.Context, req *Compose) (*Compose, error)
	List(ctx context.Context, req *ComposeList) (*ComposeList, error)
	Update(ctx context.Context, req *Compose) (*Compose, error)
}

// ComposeServiceHandler provides a Zip API Object Framework service for a
// Unikraft project.
type ComposeServiceHandler struct {
	create zip.MethodStrategy[*Compose, *Compose]
	delete zip.MethodStrategy[*Compose, *Compose]
	get    zip.MethodStrategy[*Compose, *Compose]
	list   zip.MethodStrategy[*ComposeList, *ComposeList]
	update zip.MethodStrategy[*Compose, *Compose]
}

// Create implements ComposeService
func (client *ComposeServiceHandler) Create(ctx context.Context, req *Compose) (*Compose, error) {
	return client.create.Do(ctx, req)
}

// Delete implements ComposeService
func (client *ComposeServiceHandler) Delete(ctx context.Context, req *Compose) (*Compose, error) {
	return client.delete.Do(ctx, req)
}

// Get implements ComposeService
func (client *ComposeServiceHandler) Get(ctx context.Context, req *Compose) (*Compose, error) {
	return client.get.Do(ctx, req)
}

// List implements ComposeService
func (client *ComposeServiceHandler) List(ctx context.Context, req *ComposeList) (*ComposeList, error) {
	return client.list.Do(ctx, req)
}

// Update implements ComposeService
func (client *ComposeServiceHandler) Update(ctx context.Context, req *Compose) (*Compose, error) {
	return client.update.Do(ctx, req)
}

// NewComposeServiceHandler returns a service based on an inline API
// client which essentially wraps the specific call, enabling pre- and post-
// call hooks.  This is useful for wrapping the command with decorators, for
// example, a cache, error handlers, etc.  Simultaneously, it enables access to
// the service via inline code without having to make invocations to an external
// handler.
func NewComposeServiceHandler(ctx context.Context, impl ComposeService, opts ...zip.ClientOption) (ComposeService, error) {
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

	return &ComposeServiceHandler{
		create,
		delete,
		get,
		list,
		update,
	}, nil
}

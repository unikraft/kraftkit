// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package runtime

import (
	"context"
	"fmt"
	"time"

	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/tableprinter"
	"kraftkit.sh/oci"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

var _ pack.Package = (*Runtime)(nil)

const (
	PrebuiltRegistry         = "unikraft.org"
	DefaultRuntime           = "unikraft.org/base:latest"
	DefaultKraftCloudRuntime = "index.unikraft.io/official/base:latest"
)

// NewRuntime prepares a pre-built unikernel application for use.
func NewRuntime(ctx context.Context, name string, pbopts ...RuntimeOption) (*Runtime, error) {
	runtime := Runtime{
		name: name,
	}

	for _, opt := range pbopts {
		if err := opt(&runtime); err != nil {
			return nil, err
		}
	}

	if runtime.name == "" {
		runtime.name = DefaultRuntime
	} else {
		// Return early if the user provided a custom elfloader unikernel
		// application.
		if ok, _ := unikraft.IsFileUnikraftUnikernel(runtime.name); ok {
			runtime.kernel = runtime.name
			return &runtime, nil
		}
	}

	var err error
	runtime.registry = packmanager.G(ctx)
	if runtime.registry == nil {
		runtime.registry, err = oci.NewOCIManager(ctx,
			oci.WithDetectHandler(),
		)
	} else {
		runtime.registry, err = runtime.registry.From(oci.OCIFormat)
	}
	if err != nil {
		return nil, err
	}

	// First try locally
	results, err := runtime.registry.Catalog(ctx,
		packmanager.WithName(runtime.name),
		packmanager.WithPlatform(runtime.platform),
		packmanager.WithArchitecture(runtime.architecture),
		packmanager.WithTypes(unikraft.ComponentTypeApp),
	)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		results, err = runtime.registry.Catalog(ctx,
			packmanager.WithName(runtime.name),
			packmanager.WithPlatform(runtime.platform),
			packmanager.WithArchitecture(runtime.architecture),
			packmanager.WithTypes(unikraft.ComponentTypeApp),
			packmanager.WithRemote(true),
		)
		if err != nil {
			return nil, err
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("could not find runtime")
	} else if len(results) > 1 {
		options := make([]string, len(results))
		for i, result := range results {
			options[i] = result.String()
		}
		return nil, fmt.Errorf("too many options: %v", options)
	}

	runtime.pack = results[0]
	runtime.name = results[0].Name()
	runtime.version = results[0].Version()
	runtime.source = results[0].Name()

	return &runtime, nil
}

// ID implements kraftkit.sh/pack.Package
func (runtime *Runtime) ID() string {
	return runtime.pack.ID()
}

// Columns implements kraftkit.sh/pack.Package
func (runtime *Runtime) Columns() []tableprinter.Column {
	return runtime.pack.Columns()
}

// Metadata implements kraftkit.sh/pack.Package
func (runtime *Runtime) Metadata() interface{} {
	return runtime.pack.Metadata()
}

// Size implements kraftkit.sh/pack.Package
func (runtime *Runtime) Size() int64 {
	return runtime.pack.Size()
}

// Push implements kraftkit.sh/pack.Package
func (runtime *Runtime) Push(ctx context.Context, opts ...pack.PushOption) error {
	panic("not implemented: kraftkit.sh/unikraft/runtime.Runtime.Push")
}

// Unpack implements kraftkit.sh/pack.Package
func (runtime *Runtime) Unpack(ctx context.Context, dir string) error {
	return runtime.pack.Unpack(ctx, dir)
}

// Pull implements kraftkit.sh/pack.Package
func (runtime *Runtime) Pull(ctx context.Context, opts ...pack.PullOption) error {
	return runtime.pack.Pull(ctx, opts...)
}

// Pull implements kraftkit.sh/pack.Package
func (runtime *Runtime) PulledAt(ctx context.Context) (bool, time.Time, error) {
	return runtime.pack.PulledAt(ctx)
}

func (runtime *Runtime) Delete(ctx context.Context) error {
	return runtime.pack.Delete(ctx)
}

// Save implements kraftkit.sh/pack.Package
func (runtime *Runtime) Save(ctx context.Context) error {
	return runtime.pack.Save(ctx)
}

// Format implements kraftkit.sh/unikraft.component.Component
func (runtime *Runtime) Format() pack.PackageFormat {
	return runtime.pack.Format()
}

// Architecture implements kraftkit.sh/unikraft.target.Target
func (runtime *Runtime) Architecture() arch.Architecture {
	if t, ok := runtime.pack.(target.Target); ok {
		return t.Architecture()
	}

	return nil
}

// Platform implements kraftkit.sh/unikraft.target.Target
func (runtime *Runtime) Platform() plat.Platform {
	if t, ok := runtime.pack.(target.Target); ok {
		return t.Platform()
	}

	return nil
}

// Kernel implements kraftkit.sh/unikraft.target.Target
func (runtime *Runtime) Kernel() string {
	if len(runtime.kernel) > 0 {
		return runtime.kernel
	}

	if t, ok := runtime.pack.(target.Target); ok {
		return t.Kernel()
	}

	return ""
}

// KernelDbg implements kraftkit.sh/unikraft.target.Target
func (runtime *Runtime) KernelDbg() string {
	if t, ok := runtime.pack.(target.Target); ok {
		return t.KernelDbg()
	}

	return ""
}

// Initrd implements kraftkit.sh/unikraft.target.Target
func (runtime *Runtime) Initrd() initrd.Initrd {
	if t, ok := runtime.pack.(target.Target); ok {
		return t.Initrd()
	}

	return nil
}

// Command implements kraftkit.sh/unikraft.target.Target
func (runtime *Runtime) Command() []string {
	if t, ok := runtime.pack.(target.Target); ok {
		return t.Command()
	}

	return nil
}

// ConfigFilename implements kraftkit.sh/unikraft.target.Target
func (runtime *Runtime) ConfigFilename() string {
	if t, ok := runtime.pack.(target.Target); ok {
		return t.ConfigFilename()
	}

	return ""
}

func (runtime *Runtime) AddRootfs(path string) error {
	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package elfloader

import (
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
)

type ELFLoader struct {
	registry packmanager.PackageManager
	kernel   string
	pack     pack.Package
}

var _ unikraft.Nameable = (*ELFLoader)(nil)

// Type implements kraftkit.sh/unikraft.Nameable
func (ocipack *ELFLoader) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

// Name implements kraftkit.sh/unikraft.Nameable
func (ocipack *ELFLoader) Name() string {
	return ocipack.pack.Name()
}

// Version implements kraftkit.sh/unikraft.Nameable
func (ocipack *ELFLoader) Version() string {
	return ocipack.pack.Version()
}

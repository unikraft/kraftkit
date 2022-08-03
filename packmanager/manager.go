// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft GmbH.  All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions
// are met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the distribution.
// 3. Neither the name of the copyright holder nor the names of its
//    contributors may be used to endorse or promote products derived from
//    this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package packmanager

import "kraftkit.sh/pack"

type PackageManager interface {
	// NewPackage initializes a new package
	NewPackageFromOptions(*pack.PackageOptions) ([]pack.Package, error)

	// Options allows you to view the current options.
	Options() *PackageManagerOptions

	// ApplyOptions allows one to update the options of a package manager
	ApplyOptions(...PackageManagerOption) error

	// Update retrieves and stores locally a cache of the upstream registry.
	Update() error

	// Push a package to the supported registry of the implementation.
	Push(string) error

	// Pull package(s) from the supported registry of the implementation.
	Pull(string, *pack.PullPackageOptions) ([]pack.Package, error)

	// Catalog returns all packages known to the manager via given query
	Catalog(CatalogQuery, ...pack.PackageOption) ([]pack.Package, error)

	// Add a source to the package manager
	AddSource(string) error

	// Remove a source from the package manager
	RemoveSource(string) error

	// IsCompatible checks whether the provided source is compatible with the
	// package manager
	IsCompatible(string) (PackageManager, error)

	// From is used to retrieve a sub-package manager.  For now, this is a small
	// hack used for the umbrella.
	From(string) (PackageManager, error)

	// String returns the name of the implementation.
	String() string
}

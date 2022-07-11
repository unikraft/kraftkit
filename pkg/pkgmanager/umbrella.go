// SPDX-License-Identifier: BSD-3-Clause
//
// Authors: Alexander Jung <alex@unikraft.io>
//
// Copyright (c) 2022, Unikraft UG.  All rights reserved.
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

package pkgmanager

import (
	"fmt"

	"go.unikraft.io/kit/pkg/pkg"
)

var packageManagers = make(map[string]PackageManager)

func RegisterPackageManager(manager PackageManager) error {
	if _, ok := packageManagers[manager.String()]; ok {
		return fmt.Errorf("package manager already registered: %s", manager.String())
	}

	packageManagers[manager.String()] = manager

	return nil
}

// UmbrellaManager is an ad-hoc package manager capable of cross managing any
// registered package manager.
type UmbrellaManager struct {
	opts *PackageManagerOptions
}

func NewUmbrellaManagerFromOptions(opts *PackageManagerOptions) (PackageManager, error) {
	umbrella := UmbrellaManager{
		opts: opts,
	}

	// Apply options on the umbrella manager to all "sub" registered package
	// managers
	for _, manager := range packageManagers {
		for _, o := range opts.opts {
			if err := o(manager.Options()); err != nil {
				return nil, err
			}
		}
	}

	return umbrella, nil
}

func (um UmbrellaManager) NewPackageFromOptions(*pkg.PackageOptions) (pkg.Package, error) {
	return nil, fmt.Errorf("cannot generate package from umbrella manager")
}

func (um UmbrellaManager) From(sub string) (PackageManager, error) {
	for _, manager := range packageManagers {
		if manager.String() == sub {
			return manager, nil
		}
	}

	return nil, fmt.Errorf("unknown package manager: %s", sub)
}

func (um UmbrellaManager) ApplyOptions(pmopts ...PackageManagerOption) error {
	for _, manager := range packageManagers {
		if err := manager.ApplyOptions(pmopts...); err != nil {
			return err
		}
	}

	return nil
}

// Options allows you to view the current options.
func (um UmbrellaManager) Options() *PackageManagerOptions {
	return um.opts
}

// Update retrieves and stores locally a
func (um UmbrellaManager) Update() error {
	for _, manager := range packageManagers {
		err := manager.Update()
		if err != nil {
			return err
		}
	}

	return nil
}

// Push the resulting package to the supported registry of the implementation.
func (um UmbrellaManager) Push(path string) error {
	return fmt.Errorf("not implemented: pkg.UmbrellaManager.Push")
}

// Pull a package from the support registry of the implementation.
func (um UmbrellaManager) Pull(path string, opts *PullPackageOptions) ([]pkg.Package, error) {
	var packages []pkg.Package
	for _, manager := range packageManagers {
		um.opts.Log.Trace("Pulling %s via %s...", path, manager.String())
		parcel, err := manager.Pull(path, opts)
		if err != nil {
			return nil, err
		}

		packages = append(packages, parcel...)
	}

	return packages, nil
}

// Search for a package with a given name
func (um UmbrellaManager) Search(needle string, opts *SearchPackageOptions) ([]pkg.Package, error) {
	var packages []pkg.Package
	for _, manager := range packageManagers {
		um.opts.Log.Trace("Searching \"%s\" via %s...", needle, manager.String())
		parcel, err := manager.Search(needle, opts)
		if err != nil {
			return nil, err
		}

		packages = append(packages, parcel...)
	}

	return packages, nil
}

// IsCompatible returns true always for the umbrella manager.
// TODO: Likely we should have some more sophiscated logic here to return the
// compatible package manager.
func (um UmbrellaManager) IsCompatible(resource string) bool {
	return true
}

// String returns the name of the implementation.
func (um UmbrellaManager) String() string {
	return "umbrella"
}

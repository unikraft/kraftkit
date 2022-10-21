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

import (
	"context"
	"fmt"

	"kraftkit.sh/pack"
)

var packageManagers = make(map[pack.ContextKey]PackageManager)

const UmbrellaContext pack.ContextKey = "umbrella"

func PackageManagers() map[pack.ContextKey]PackageManager {
	return packageManagers
}

func RegisterPackageManager(ctxk pack.ContextKey, manager PackageManager) error {
	if _, ok := packageManagers[ctxk]; ok {
		return fmt.Errorf("package manager already registered: %s", manager.Format())
	}

	packageManagers[ctxk] = manager

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

func (um UmbrellaManager) NewPackageFromOptions(ctx context.Context, opts *pack.PackageOptions) ([]pack.Package, error) {
	var packages []pack.Package
	for _, manager := range packageManagers {
		packed, err := manager.NewPackageFromOptions(ctx, opts)
		if err != nil {
			return packages, err
		}

		packages = append(packages, packed...)
	}

	return packages, nil
}

func (um UmbrellaManager) From(sub string) (PackageManager, error) {
	for _, manager := range packageManagers {
		if manager.Format() == sub {
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

func (um UmbrellaManager) AddSource(source string) error {
	for _, manager := range packageManagers {
		um.opts.Log.Trace("Adding source %s via %s...", source, manager.Format())
		err := manager.AddSource(source)
		if err != nil {
			return err
		}
	}

	return nil
}

func (um UmbrellaManager) RemoveSource(source string) error {
	for _, manager := range packageManagers {
		um.opts.Log.Trace("Removing source %s via %s...", source, manager.Format())
		err := manager.RemoveSource(source)
		if err != nil {
			return err
		}
	}

	return nil
}

// Push the resulting package to the supported registry of the implementation.
func (um UmbrellaManager) Push(path string) error {
	return fmt.Errorf("not implemented: pack.UmbrellaManager.Push")
}

// Pull a package from the support registry of the implementation.
func (um UmbrellaManager) Pull(path string, opts *pack.PullPackageOptions) ([]pack.Package, error) {
	var packages []pack.Package
	for _, manager := range packageManagers {
		um.opts.Log.Trace("Pulling %s via %s...", path, manager.Format())
		parcel, err := manager.Pull(path, opts)
		if err != nil {
			return nil, err
		}

		packages = append(packages, parcel...)
	}

	return packages, nil
}

func (mm UmbrellaManager) Catalog(query CatalogQuery, popts ...pack.PackageOption) ([]pack.Package, error) {
	var packages []pack.Package
	for _, manager := range packageManagers {
		pack, err := manager.Catalog(query, popts...)
		if err != nil {
			return nil, err
		}

		packages = append(packages, pack...)
	}

	return packages, nil
}

// IsCompatible iterates through all package managers and returns the first
// package manager which is compatible with the provided source
func (mm UmbrellaManager) IsCompatible(source string) (PackageManager, error) {
	var err error
	var pm PackageManager
	for _, manager := range packageManagers {
		pm, err = manager.IsCompatible(source)
		if err == nil {
			return pm, nil
		}
	}

	return nil, fmt.Errorf("cannot find compatible package manager for source: %s", source)
}

func (um UmbrellaManager) Format() string {
	return string(UmbrellaContext)
}

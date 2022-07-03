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

package manifest

import (
	"context"
	"fmt"
	"net/http"

	"go.unikraft.io/kit/config"
	"go.unikraft.io/kit/pkg/iostreams"
	"go.unikraft.io/kit/pkg/pkg"
	"go.unikraft.io/kit/pkg/pkgmanager"
)

type ManifestManager struct {
	dataDir func() string
	client  *http.Client
	config  *config.Config
	io      *iostreams.IOStreams
	opts    *pkgmanager.PackageManagerOptions
}

func init() {
	options, err := pkgmanager.NewPackageManagerOptions(
		context.TODO(),
	)
	if err != nil {
		panic(fmt.Sprintf("could not register package manager options: %s", err))
	}

	manager, err := NewManifestPackageManagerFromOptions(options)
	if err != nil {
		panic(fmt.Sprintf("could not register package manager: %s", err))
	}

	// Register a new pkg.Package type
	pkgmanager.RegisterPackageManager(manager)
}

func NewManifestPackageManagerFromOptions(opts *pkgmanager.PackageManagerOptions) (pkgmanager.PackageManager, error) {
	return ManifestManager{
		opts: opts,
	}, nil
}

// NewPackage initializes a new package
func (mm ManifestManager) NewPackageFromOptions(opts *pkg.PackageOptions) (pkg.Package, error) {
	return NewPackageFromOptions(opts)
}

// Options allows you to view the current options.
func (mm ManifestManager) Options() *pkgmanager.PackageManagerOptions {
	return mm.opts
}

// Update retrieves and stores locally a cache of the upstream manifest registry.
func (mm ManifestManager) Update() error {
	return fmt.Errorf("not implemented pkg.ManifestManager.Update")
}

// Push the resulting package to the supported registry of the implementation.
func (mm ManifestManager) Push(path string) error {
	return fmt.Errorf("not implemented pkg.ManifestManager.Pushh")
}

// Pull a package from the support registry of the implementation.
func (mm ManifestManager) Pull(path string, opts *pkgmanager.PullPackageOptions) ([]pkg.Package, error) {
	return nil, fmt.Errorf("not implemented pkg.ManifestManager.Pull")
}

func (um ManifestManager) From(sub string) (pkgmanager.PackageManager, error) {
	return nil, fmt.Errorf("method not applicable to manifest manager")
}

// Search for a package with a given name
func (mm ManifestManager) Search(needle string, opts *pkgmanager.SearchPackageOptions) ([]pkg.Package, error) {
	return nil, fmt.Errorf("not implemented pkg.ManifestManager.Search")
}

// String returns the name of the implementation.
func (mm ManifestManager) String() string {
	return "manifest"
}

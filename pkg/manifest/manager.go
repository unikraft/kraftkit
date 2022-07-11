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
	"os"
	"path/filepath"
	"time"

	"github.com/gobwas/glob"

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
func (mm ManifestManager) NewPackageFromOptions(opts *pkg.PackageOptions) ([]pkg.Package, error) {
	mm.opts.Log.Infof("initializing new manifest package...")
	pack, err := NewPackageFromOptions(opts)
	return []pkg.Package{pack}, err
}

// Options allows you to view the current options.
func (mm ManifestManager) Options() *pkgmanager.PackageManagerOptions {
	return mm.opts
}

func (mm ManifestManager) ApplyOptions(pmopts ...pkgmanager.PackageManagerOption) error {
	for _, opt := range pmopts {
		if err := opt(mm.opts); err != nil {
			return err
		}
	}

	return nil
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

func (mm ManifestManager) Catalog(query pkgmanager.CatalogQuery) ([]pkg.Package, error) {
	index, err := NewManifestIndexFromFile(mm.LocalManifestIndex())
	if err != nil {
		return nil, err
	}

	mopts := []ManifestOption{
		WithAuthConfig(mm.Options().ConfigManager.Config.Auth),
		WithSourcesRootDir(mm.Options().ConfigManager.Config.Paths.Sources),
		WithLogger(mm.Options().Log),
	}

	var allManifests []*Manifest

	for _, manifest := range index.Manifests {
		if len(manifest.Manifest) > 0 {
			manifests, err := findManifestsFromSource(mm.LocalManifestsDir(), manifest.Manifest, mopts)
			if err != nil {
				return nil, err
			}

			allManifests = append(allManifests, manifests...)
		} else {
			allManifests = append(allManifests, manifest)
		}
	}

	var packages []pkg.Package
	var g glob.Glob

	if len(query.Name) > 0 {
		g = glob.MustCompile(query.Name)
	}

	for _, manifest := range allManifests {
		if len(query.Types) > 0 {
			found := false
			for _, t := range query.Types {
				if manifest.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if len(query.Name) > 0 && !g.Match(manifest.Name) {
			continue
		}

		var versions []string
		if len(query.Version) > 0 {
			for _, version := range manifest.Versions {
				if version.Version == query.Version {
					versions = append(versions, version.Version)
					break
				}
			}
			if len(versions) == 0 {
				for _, channel := range manifest.Channels {
					if channel.Name == query.Version {
						versions = append(versions, channel.Name)
						break
					}
				}
			}

			if len(versions) == 0 {
				break
			}
		}

		if len(versions) > 0 {
			for _, version := range versions {
				pack, err := NewPackageWithVersion(manifest, version)
				if err != nil {
					mm.opts.Log.Warnf("%v", err)
					continue
					// TODO: Config option for fast-fail?
					// return nil, err
				}

				packages = append(packages, pack)
			}
		} else {
			packs, err := NewPackageFromManifest(manifest)
			if err != nil {
				mm.opts.Log.Warnf("%v", err)
				continue
				// TODO: Config option for fast-fail?
				// return nil, err
			}

			packages = append(packages, packs)
		}
	}

	for i := range packages {
		packages[i].ApplyOptions(
			pkg.WithLogger(mm.Options().Log),
		)
	}

	return packages, nil
}

func (mm ManifestManager) IsCompatible(source string) (pkgmanager.PackageManager, error) {
	if _, err := NewProvider(source); err != nil {
		return nil, fmt.Errorf("incompatible source")
	}

	return mm, nil
}

// String returns the name of the implementation.
func (mm ManifestManager) String() string {
	return "manifest"
}

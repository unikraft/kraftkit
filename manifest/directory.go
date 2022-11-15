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

package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/app"
)

type DirectoryProvider struct {
	typ   unikraft.ComponentType
	name  string
	dir   string
	mopts []ManifestOption
}

// NewDirectoryProvider attempts to parse a provided path as a Unikraft
// "microlibirary" directory
func NewDirectoryProvider(path string, mopts ...ManifestOption) (Provider, error) {
	if f, err := os.Stat(path); err != nil || (err == nil && !f.IsDir()) {
		return nil, fmt.Errorf("could not access directory '%s': %v", path, err)
	}

	// Determine the type preemptively
	dirname := filepath.Base(path)
	t, n, _, err := unikraft.GuessTypeNameVersion(dirname)
	if err != nil || t == unikraft.ComponentTypeUnknown {
		for _, f := range app.DefaultFileNames {
			if f, err := os.Stat(filepath.Join(path, f)); err == nil && f.Size() > 0 {
				t = unikraft.ComponentTypeApp
				break
			}
		}
	}

	// TODO: This is a very lightweight way of determining whether the provided
	// directory is a microlibrary.  In reality, a `Config.uk` and `Makefile.uk`
	// could also exist within an application or external platform.  Use
	// introspection to learn the registration used to determine the type/
	if t == unikraft.ComponentTypeUnknown {
		if f, err := os.Stat(filepath.Join(path, unikraft.Config_uk)); err == nil && f.Size() > 0 {
			t = unikraft.ComponentTypeLib
		}
	}

	if t == unikraft.ComponentTypeUnknown {
		return nil, fmt.Errorf("unknown type for directory: %s", path)
	}

	return DirectoryProvider{
		typ:   t,
		name:  n,
		dir:   path,
		mopts: mopts,
	}, nil
}

func (dp DirectoryProvider) Manifests() ([]*Manifest, error) {
	manifest := &Manifest{
		Type:     dp.typ,
		Name:     dp.name,
		Provider: dp,
		Origin:   dp.dir,
		Channels: []ManifestChannel{
			{
				Name:     "default",
				Default:  true,
				Resource: dp.dir,
			},
		},
	}

	for _, opt := range dp.mopts {
		if err := opt(manifest); err != nil {
			return nil, fmt.Errorf("could not apply option: %v", err)
		}
	}

	// Force the local back to the original directory
	manifest.Channels[0].Local = dp.dir

	return []*Manifest{manifest}, nil
}

func (dp DirectoryProvider) PullPackage(manifest *Manifest, popts *pack.PackageOptions, ppopts *pack.PullPackageOptions) error {
	if len(ppopts.Workdir()) == 0 {
		return fmt.Errorf("cannot pull without without working directory")
	}

	// The directory provider only has one channel, exploit this knowledge
	if len(manifest.Channels) != 1 {
		return fmt.Errorf("cannot determine channel for directory provider")
	}

	local, err := unikraft.PlaceComponent(
		ppopts.Workdir(),
		manifest.Type,
		manifest.Name,
	)
	if err != nil {
		return fmt.Errorf("could not place component package: %s", err)
	}

	f, err := os.Lstat(local)
	if err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
			return err
		}
	} else if err == nil && f.IsDir() {
		return fmt.Errorf("local directory already exists: %s", local)
	} else if err == nil && f.Mode()&os.ModeSymlink == os.ModeSymlink {
		if err := os.Remove(local); err != nil {
			return fmt.Errorf("could not remove symlink: %v", err)
		}
	}

	// Simply generate a symbolic link to the directory resource
	if err := os.Symlink(manifest.Channels[0].Resource, local); err != nil {
		return fmt.Errorf("could not copy directory: %v", err)
	}

	return nil
}

func (dp DirectoryProvider) String() string {
	return "dir"
}

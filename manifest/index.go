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
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v2"
	"kraftkit.sh/pack"
)

type ManifestIndex struct {
	Name        string      `yaml:"name,omitempty"`
	LastUpdated time.Time   `yaml:"last_updated"`
	Manifests   []*Manifest `yaml:"manifests"`
}

type ManifestIndexProvider struct {
	path  string
	index *ManifestIndex
	mopts []ManifestOption
}

// NewManifestIndexProvider accepts an input path which is checked against first
// a local file on disk and then a remote URL.  If either of these checks pass,
// the Provider is instantiated since the path does indeed represent a
// ManifestIndex.
func NewManifestIndexProvider(path string, mopts ...ManifestOption) (Provider, error) {
	index, err := NewManifestIndexFromFile(path, mopts...)
	if err == nil {
		return ManifestIndexProvider{
			path:  path,
			index: index,
			mopts: mopts,
		}, nil
	}

	index, err = NewManifestIndexFromURL(path, mopts...)
	if err == nil {
		return ManifestIndexProvider{
			path:  path,
			index: index,
			mopts: mopts,
		}, nil
	}

	return nil, fmt.Errorf("provided path is not a manifest index: %s", path)
}

func (mip ManifestIndexProvider) Manifests() ([]*Manifest, error) {
	var manifests []*Manifest

	for _, manifest := range mip.index.Manifests {
		next, err := findManifestsFromSource(mip.path, manifest.Manifest, mip.mopts)
		if err != nil {
			return nil, err
		}

		manifests = append(manifests, next...)
	}

	return manifests, nil
}

func (mip ManifestIndexProvider) PullPackage(manifest *Manifest, popts *pack.PackageOptions, ppopts *pack.PullPackageOptions) error {
	return fmt.Errorf("not implemented: manifest.ManifestIndexProvider.PullPackage")
}

func (mip ManifestIndexProvider) String() string {
	return "index"
}

// NewManifestIndexFromBytes parses a byte array of a YAML representing a
// manifest index
func NewManifestIndexFromBytes(raw []byte, mopts ...ManifestOption) (*ManifestIndex, error) {
	index := &ManifestIndex{}

	if err := yaml.Unmarshal(raw, index); err != nil {
		return nil, err
	}

	if index.Manifests == nil {
		return nil, fmt.Errorf("nothing found in manifest index")
	}

	for i, manifest := range index.Manifests {
		for _, o := range mopts {
			if err := o(manifest); err != nil {
				return nil, err
			}
		}

		index.Manifests[i] = manifest
	}

	return index, nil
}

func NewManifestIndexFromFile(path string, mopts ...ManifestOption) (*ManifestIndex, error) {
	f, err := os.Stat(path)
	if err != nil {
		return nil, err
	} else if f.Size() == 0 {
		return nil, fmt.Errorf("manifest index is empty: %s", path)
	}

	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return NewManifestIndexFromBytes(contents, mopts...)
}

// NewManifestFromURL retrieves a provided path as a ManifestIndex from a remote
// location over HTTP
func NewManifestIndexFromURL(path string, mopts ...ManifestOption) (*ManifestIndex, error) {
	_, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	resp, err := http.Head(path)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest index not found: %s", path)
	}

	resp, err = http.Get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received %d error when retreiving: %s", resp.StatusCode, path)
	}

	// Check if we're directly pointing to a compatible manifest file
	ext := filepath.Ext(path)
	if ext != ".yml" && ext != ".yaml" {
		return nil, fmt.Errorf("unsupported manifest index extension for path: %s", path)
	}

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	providerRequestCache = contents

	index, err := NewManifestIndexFromBytes(contents, mopts...)
	if err != nil {
		return nil, err
	}

	return index, nil
}

func (mi *ManifestIndex) WriteToFile(path string) error {
	// Open the file (create if not present)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("could not open file: %v", err)
	}

	defer f.Close()

	contents, err := yaml.Marshal(mi)
	if err != nil {
		return err
	}

	if err := f.Truncate(0); err != nil {
		return err
	}

	_, err = f.Write(contents)
	if err != nil {
		return err
	}

	return nil
}

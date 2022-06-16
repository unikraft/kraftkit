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
	"fmt"

	"go.unikraft.io/kit/pkg/unikraft"
	"gopkg.in/yaml.v2"
)

type Manifest struct {
	// Name of the entity which this manifest represents
	Name string `yaml:"name"`

	// Type of entity which this manifest represetns
	Type unikraft.ComponentType `yaml:"type"`

	// Manifest is used to point to remote manifest, allowing the manifest itself
	// to be retrieved by indirection.  Manifest is XOR with Versions and should
	// be back-propagated.
	Manifest string `yaml:"manifest,omitempty"`

	// Description of what this manifest represents
	Description string `yaml:"description,omitempty"`

	// Channels provides multiple ways to retrieve versions.  Classically this is
	// a separation between "staging" and "stable"
	Channels []ManifestChannel `yaml:"channels,omitempty"`

	// Versions
	Versions []ManifestVersion `yaml:"versions,omitempty"`

	// SourceOrigin is original location of where this manifest was found
	SourceOrigin string `yaml:"-"`
}

// NewManifestFromBytes parses a byte array of a YAML representing a manifest
func NewManifestFromBytes(raw []byte) (*Manifest, error) {
	manifest := &Manifest{}
	if err := yaml.Unmarshal(raw, manifest); err != nil {
		return nil, err
	}

	if len(manifest.Name) == 0 {
		return nil, fmt.Errorf("unset name in manifest")
	} else if len(manifest.Type) == 0 {
		return nil, fmt.Errorf("unset type in manifest")
	}

	return manifest, nil
}

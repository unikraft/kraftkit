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
	"path/filepath"

	"go.unikraft.io/kit/config"
	"go.unikraft.io/kit/log"
	"go.unikraft.io/kit/unikraft"
)

type ManifestOption func(m *Manifest) error

func WithAuthConfig(auths map[string]config.AuthConfig) ManifestOption {
	return func(m *Manifest) error {
		m.auths = auths
		return nil
	}
}

func WithLogger(l log.Logger) ManifestOption {
	return func(m *Manifest) error {
		m.log = l
		return nil
	}
}

// WithSourcesRootDir is an option wich helps find cached Manifest Channel or
// Version resources.  When set to a directory, the fixed structure of this
// directory should allow us to look up (and also store) resources here for
// later use.
func WithSourcesRootDir(dir string) ManifestOption {
	return func(m *Manifest) error {
		// See: https://github.com/golang/go/wiki/CommonMistakes#using-reference-to-loop-iterator-variable
		dir := dir

		if m.Type != unikraft.ComponentTypeCore {
			dir = filepath.Join(dir, m.Type.Plural())
		}

		for i, channel := range m.Channels {
			ext := filepath.Ext(channel.Resource)
			if ext == ".gz" {
				ext = ".tar.gz"
			}

			m.Channels[i].Local = filepath.Join(
				dir, m.Name+"-"+channel.Name+ext,
			)
		}

		for i, version := range m.Versions {
			ext := filepath.Ext(version.Resource)
			if ext == ".gz" {
				ext = ".tar.gz"
			}

			m.Versions[i].Local = filepath.Join(
				dir, m.Name+"-"+version.Version+ext,
			)
		}

		return nil
	}
}

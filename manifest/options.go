// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package manifest

import (
	"path/filepath"

	"kraftkit.sh/config"
	"kraftkit.sh/unikraft"
)

type ManifestOption func(m *Manifest) error

func WithAuthConfig(auths map[string]config.AuthConfig) ManifestOption {
	return func(m *Manifest) error {
		m.auths = auths
		return nil
	}
}

// WithSourcesRootDir is an option which helps find cached Manifest Channel or
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

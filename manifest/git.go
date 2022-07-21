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
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	gitplumbing "github.com/go-git/go-git/v5/plumbing"
	"go.unikraft.io/kit/unikraft"
)

type GitProvider struct {
	repo   string
	remote *git.Remote
	refs   []*gitplumbing.Reference
	mopts  []ManifestOption
}

// NewGitProvider attempts to parse a provided path as a Git repository
func NewGitProvider(path string, mopts ...ManifestOption) (Provider, error) {
	// Check if the remote URL is a Git repository
	remote := git.NewRemote(nil, &gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{path},
	})

	// If this is a valid Git repository then let's generate a Manifest based on
	// what we can read from the remote
	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not access access path as Git repository: %s", path)
	}

	return GitProvider{
		repo:   path,
		remote: remote,
		refs:   refs,
		mopts:  mopts,
	}, nil
}

// probeChannels is an internal method which matches Git branches for the
// repository and uses this as a ManifestChannel
func (gp GitProvider) probeChannels() ([]ManifestChannel, error) {
	var channels []ManifestChannel

	// This is unikraft-centric ettiquette where there exists two branches:
	// "stable" and "staging".  If these two channels exist, then later on we'll
	// update the "stable" channel to be the default.
	haveStaging := false
	haveStable := false

	for _, ref := range gp.refs {
		// Branches are channels
		if ref.Name().IsBranch() {
			channel := ManifestChannel{
				Name:     ref.Name().Short(),
				Resource: gp.repo,
			}

			if ref.Name().Short() == "staging" {
				haveStaging = true
			} else if ref.Name().Short() == "stable" {
				haveStable = true
			}

			channels = append(channels, channel)
			continue
		}
	}

	if haveStaging && haveStable {
		for i, channel := range channels {
			if channel.Name == "stable" {
				channel.Default = true
				channels[i] = channel
			}
		}
	}

	return channels, nil
}

// probeVersions is an internal method which matches Git tags for the repository
// and uses this as a ManifestVersion
func (gp GitProvider) probeVersions() ([]ManifestVersion, error) {
	var versions []ManifestVersion

	for _, ref := range gp.refs {
		// Tags are versions
		if ref.Name().IsTag() {
			ver := ref.Name().Short()
			version := ManifestVersion{
				Version:  ver,
				Resource: gp.repo,
			}

			// This is a unikraft-centric ettiquette where the Unikraft core
			// repository's version is referenced via the `RELEASE-` prefix.  If
			// this is the case, we can select the Git SHA as the version and
			// subsequently set the Unikraft core version in one.
			if strings.HasPrefix(ver, "RELEASE-") {
				version.Unikraft = strings.TrimPrefix(ver, "RELEASE-")
				version.Version = ref.Hash().String()[:7]
				version.Type = ManifestVersionGitSha
			}

			versions = append(versions, version)
		}
	}

	return versions, nil
}

func (gp GitProvider) Manifests() ([]*Manifest, error) {
	base := filepath.Base(gp.repo)
	ext := filepath.Ext(gp.repo)
	if len(ext) > 0 {
		base = strings.TrimSuffix(base, ext)
	}

	t, n, _ := unikraft.GuessTypeNameVersion(base)

	manifest := &Manifest{
		Type:    t,
		Name:    n,
		GitRepo: gp.repo,
	}

	for _, opt := range gp.mopts {
		if err := opt(manifest); err != nil {
			return nil, fmt.Errorf("could not apply option: %v", err)
		}
	}

	channels, err := gp.probeChannels()
	if err != nil {
		return nil, err
	}

	manifest.log.Infof("probing %s", gp.repo)

	manifest.Channels = append(manifest.Channels, channels...)

	versions, err := gp.probeVersions()
	if err != nil {
		return nil, err
	}

	manifest.Versions = append(manifest.Versions, versions...)

	return []*Manifest{manifest}, nil
}

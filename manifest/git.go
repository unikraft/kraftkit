// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	gitplumbing "github.com/go-git/go-git/v5/plumbing"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	giturl "github.com/kubescape/go-git-url"

	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
)

type GitProvider struct {
	repo   string
	remote *git.Remote
	refs   []*gitplumbing.Reference
	mopts  []ManifestOption
	branch string
	ctx    context.Context
}

// NewGitProvider attempts to parse a provided path as a Git repository
func NewGitProvider(ctx context.Context, path string, mopts ...ManifestOption) (Provider, error) {
	isSSH := false
	fullpath := path
	if isSSHURL(path) {
		isSSH = true

		// This is a quirk of go-git, if we have determined it was an SSH path and
		// it does not contain the prefix, we should include it so it can be
		// recognised internally by the module.
		if strings.HasPrefix(path, "git@") {
			fullpath = "ssh://" + path
		}
	}

	gitURL, err := giturl.NewGitURL(fullpath) // initialize and parse the URL
	if err != nil {
		return nil, err
	}

	// Check if the remote URL is a Git repository
	remote := git.NewRemote(nil, &gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{fullpath},
	})

	lopts := &git.ListOptions{}
	tempm := &Manifest{}

	for _, opt := range mopts {
		if err := opt(tempm); err != nil {
			return nil, fmt.Errorf("could not apply option: %v", err)
		}
	}

	if isSSH {
		user := gitURL.GetURL().User.Username()
		if user == "" {
			user = "git"
		}

		lopts.Auth, err = gitssh.DefaultAuthBuilder(user)
		if err != nil {
			return nil, err
		}
	} else if auth, ok := tempm.auths[gitURL.GetHostName()]; ok {
		if len(auth.User) > 0 {
			lopts.Auth = &githttp.BasicAuth{
				Username: auth.User,
				Password: auth.Token,
			}
		} else if len(auth.Token) > 0 {
			lopts.Auth = &githttp.TokenAuth{
				Token: auth.Token,
			}
		}
	}

	// If this is a valid Git repository then let's generate a Manifest based on
	// what we can read from the remote
	refs, err := remote.ListContext(ctx, lopts)
	if err != nil {
		return nil, fmt.Errorf("could not list remote git repository: %v", err)
	}

	return GitProvider{
		repo:   path,
		remote: remote,
		refs:   refs,
		mopts:  mopts,
		branch: gitURL.GetBranchName(),
		ctx:    ctx,
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
			if len(gp.branch) > 0 && ref.Name().Short() != gp.branch {
				continue
			}

			channel := ManifestChannel{
				Name:     ref.Name().Short(),
				Resource: gp.repo,
			}

			// Unikraft-centric naming conventions
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
			if len(gp.branch) > 0 && ref.Name().Short() != gp.branch {
				continue
			}

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

	t, n, _, err := unikraft.GuessTypeNameVersion(base)
	if err != nil {
		return nil, err
	}

	manifest := &Manifest{
		Type:     t,
		Name:     n,
		Origin:   gp.repo,
		Provider: gp,
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

	log.G(gp.ctx).Infof("probing %s", gp.repo)

	manifest.Channels = append(manifest.Channels, channels...)

	versions, err := gp.probeVersions()
	if err != nil {
		return nil, err
	}

	manifest.Versions = append(manifest.Versions, versions...)

	// TODO: This is the correct place to apply the options.  We do it earlier to
	// access the logger.  The same issue appears in github.go.  The logger
	// interface needs to be replaced with a contextualised version, see:
	// https://github.com/unikraft/kraftkit/issues/74
	for _, opt := range gp.mopts {
		if err := opt(manifest); err != nil {
			return nil, fmt.Errorf("could not apply option: %v", err)
		}
	}

	// TODO: Set the latest version
	// if len(manifest.Versions) > 0 {
	// }

	return []*Manifest{manifest}, nil
}

func (gp GitProvider) PullManifest(ctx context.Context, manifest *Manifest, popts ...pack.PullOption) error {
	if useGit {
		return pullGit(ctx, manifest, popts...)
	}

	return pullArchive(ctx, manifest, popts...)
}

func (gp GitProvider) String() string {
	return "git"
}

// isSSHURL determines if the provided URL forms an SSH connection
func isSSHURL(path string) bool {
	for _, prefix := range []string{
		"ssh://",
		"ssh+git://",
		"git+ssh://",
		"git@",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

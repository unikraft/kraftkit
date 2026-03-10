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
	mopts  *ManifestOptions
	branch string
	ctx    context.Context
}

// NewGitProvider attempts to parse a provided path as a Git repository
func NewGitProvider(ctx context.Context, path string, opts ...ManifestOption) (Provider, error) {
	isSSH := false
	fullpath := path

	if strings.HasPrefix(path, "github.com/") {
		fullpath = "https://" + path
	} else if isSSHURL(path) {
		isSSH = true

		// This is a quirk of go-git, if we have determined it was an SSH path and
		// it does not contain the prefix, we should include it so it can be
		// recognised internally by the module.
		if strings.HasPrefix(path, "git@") {
			// Convert SCP-style git@HOST:path to ssh://git@HOST/path so that
			// URL parsers (giturl, go-git) handle it correctly.
			scp := strings.TrimPrefix(path, "git@")
			scp = strings.Replace(scp, ":", "/", 1)
			fullpath = "ssh://git@" + scp
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
	provider := GitProvider{
		repo:   path,
		remote: remote,
		mopts:  NewManifestOptions(opts...),
		branch: gitURL.GetBranchName(),
		ctx:    ctx,
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
	} else if auth, ok := provider.mopts.auths[gitURL.GetHostName()]; ok {
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
	provider.refs, err = remote.ListContext(ctx, lopts)
	if err != nil && strings.Contains(err.Error(), "remote repository is empty") {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("could not list remote git repository: %v", err)
	}

	return &provider, nil
}

// probeChannels is an internal method which matches Git branches for the
// repository and uses this as a ManifestChannel
func (gp *GitProvider) probeChannels() []ManifestChannel {
	var channels []ManifestChannel

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

			if ref.Name().Short() == gp.mopts.defaultChannelName {
				channel.Default = true
			}

			channels = append(channels, channel)
			continue
		}
	}

	return channels
}

// probeVersions is an internal method which matches Git tags for the repository
// and uses this as a ManifestVersion
func (gp *GitProvider) probeVersions() []ManifestVersion {
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

	return versions
}

func (gp *GitProvider) Manifests() ([]*Manifest, error) {
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

	log.G(gp.ctx).Infof("probing %s", gp.repo)

	manifest.Channels = append(manifest.Channels, gp.probeChannels()...)
	manifest.Versions = append(manifest.Versions, gp.probeVersions()...)

	// TODO: Set the latest version
	// if len(manifest.Versions) > 0 {
	// }

	return []*Manifest{manifest}, nil
}

func (gp *GitProvider) PullChannel(ctx context.Context, manifest *Manifest, channel *ManifestChannel, popts ...pack.PullOption) error {
	manifest.mopts = gp.mopts

	if ForceGit {
		return pullGit(ctx, manifest, popts...)
	}

	return pullArchive(ctx, manifest, channel.Resource, channel.Sha256, popts...)
}

func (gp *GitProvider) PullVersion(ctx context.Context, manifest *Manifest, version *ManifestVersion, popts ...pack.PullOption) error {
	manifest.mopts = gp.mopts

	if ForceGit {
		return pullGit(ctx, manifest, popts...)
	}

	return pullArchive(ctx, manifest, version.Resource, version.Sha256, popts...)
}

func (gp *GitProvider) DeleteManifest(context.Context) error {
	return fmt.Errorf("not implemented: manifest.GitProvider.DeleteManifest")
}

func (gp *GitProvider) String() string {
	return "git"
}

func (gp *GitProvider) MarshalJSON() ([]byte, error) {
	return []byte("\"git\""), nil
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

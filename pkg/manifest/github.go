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
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gobwas/glob"
	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"

	"go.unikraft.io/kit/internal/ghrepo"
	"go.unikraft.io/kit/pkg/log"
	"go.unikraft.io/kit/pkg/unikraft"
)

type GitHubProvider struct {
	repo   ghrepo.Interface
	mopts  []ManifestOption
	client *github.Client
	log    log.Logger
}

// NewGitHubProvider attempts to parse the input path as a location provided on
// GitHub.  Additional context for authentication is necessary to use this
// provider if the location is held within a private repository.  Otherwise we
// can both probe GitHub's API as well as exploit the fact that it is a Git
// repository to retrieve information and deliver information as a Manifest
// format.
func NewGitHubProvider(path string, mopts ...ManifestOption) (Provider, error) {
	repo, err := ghrepo.NewFromURL(path)
	if err != nil {
		return nil, err
	}

	// Cheap hack to get authentication details for GitHub and the logger
	manifest := &Manifest{}
	for _, o := range mopts {
		if err := o(manifest); err != nil {
			return nil, err
		}
	}

	client := github.NewClient(nil)
	if ghauth, ok := manifest.Auths()[repo.RepoHost()]; ok {
		ctx := context.TODO()

		if !ghauth.VerifySSL {
			insecureClient := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}

			ctx = context.WithValue(
				context.TODO(),
				oauth2.HTTPClient,
				insecureClient,
			)
		}

		oauth2Client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(
			&oauth2.Token{
				AccessToken: ghauth.Token,
			},
		))

		// Is this a GitHub enterprise host?
		if repo.RepoHost() != "github.com" {
			endpoint, err := url.Parse(repo.RepoHost())
			if err != nil {
				return nil, fmt.Errorf("failed to parse v3 endpoint: %s", err)
			}

			client, err = github.NewEnterpriseClient(
				endpoint.String(),
				endpoint.String(),
				oauth2Client,
			)
			if err != nil {
				return nil, err
			}
		} else {
			client = github.NewClient(oauth2Client)
		}
	}

	return GitHubProvider{
		repo:   repo,
		mopts:  mopts,
		client: client,
		log:    manifest.log,
	}, nil
}

func (ghp GitHubProvider) Manifests() ([]*Manifest, error) {
	// Is this a wildcard? E.g. lib-*?
	if strings.HasSuffix(ghp.repo.RepoName(), "*") {
		return ghp.manifestsFromWildcard()
	}

	// Ultimately, since this is Git, we can use the GitProvider, and update the
	// path to the resource with a known location
	manifest, err := gitProviderFromGitHub(ghp.repo, ghp.mopts...)
	if err != nil {
		return nil, err
	}

	return []*Manifest{manifest}, nil
}

// manifestsFromWildcard is an internal method which is called by Manifests to
// parse a GitHub source with a wildcard repository name, e.g. lib-*
func (ghp GitHubProvider) manifestsFromWildcard() ([]*Manifest, error) {
	if !strings.HasSuffix(ghp.repo.RepoName(), "*") {
		return nil, fmt.Errorf("not a wildcard")
	}

	var repos []*github.Repository
	opts := github.ListOptions{}
	g := glob.MustCompile(ghp.repo.RepoName())

	for {
		more, resp, err := ghp.client.Repositories.ListByOrg(
			context.TODO(),
			ghp.repo.RepoOwner(),
			&github.RepositoryListByOrgOptions{
				ListOptions: opts,
			},
		)
		if err != nil {
			return nil, err
		}

		for _, repo := range more {
			if !g.Match(*repo.Name) {
				continue
			}

			ghp.log.Infof("found via wildcard %s", *repo.HTMLURL)
			repos = append(repos, repo)
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	var manifests []*Manifest

	for _, repo := range repos {
		ghr, err := ghrepo.NewFromURL(*repo.HTMLURL)
		if err != nil {
			return nil, err
		}

		manifest, err := gitProviderFromGitHub(ghr, ghp.mopts...)
		if err != nil {
			return nil, err
		}

		// Populate with GitHub API-centric information
		if repo.Description != nil {
			manifest.Description = *repo.Description
		}

		typ, _, err := unikraft.GuessNameAndType(*repo.Name)
		if err != nil {
			return nil, err
		}

		manifest.Type = typ

		manifests = append(manifests, manifest)
	}

	return manifests, nil
}

// gitProviderFromGitHub is a cheap internal hack since we know that GitProvider
// simply returns a slice containing exactly 1 Manifest so as to be complicit
// with the Provider interface.  We can exploit this knowledge by accepting an
// input GitHub repository and making the necessary known adjustments to the
// channel and version Resource attribute.
func gitProviderFromGitHub(repo ghrepo.Interface, mopts ...ManifestOption) (*Manifest, error) {
	// Ultimately, since this is Git, we can use the GitProvider, and update the
	// path to the resource with a known location
	provider, err := NewGitProvider(ghrepo.GenerateRepoURL(repo, ""), mopts...)
	if err != nil {
		return nil, err
	}

	manifests, err := provider.Manifests()
	if err != nil {
		return nil, err
	}

	// Here's the "hack"
	manifest := manifests[0]

	for j, channel := range manifest.Channels {
		channel.Resource = ghrepo.BranchArchive(repo, channel.Name)
		manifest.Channels[j] = channel
	}

	for j, version := range manifest.Versions {
		if version.Type == ManifestVersionGitSha {
			version.Resource = ghrepo.SHAArchive(repo, version.Version)
		} else {
			version.Resource = ghrepo.TagArchive(repo, version.Version)
		}

		manifest.Versions[j] = version
	}

	return manifest, nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package manifest

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gobwas/glob"
	"github.com/google/go-github/v32/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"kraftkit.sh/config"
	"kraftkit.sh/internal/ghrepo"
	"kraftkit.sh/log"
	"kraftkit.sh/pack"
	"kraftkit.sh/unikraft"
)

type GitHubProvider struct {
	path   string
	repo   ghrepo.Interface
	mopts  *ManifestOptions
	client *github.Client
	branch string
	ctx    context.Context
}

// NewGitHubProvider attempts to parse the input path as a location provided on
// GitHub.  Additional context for authentication is necessary to use this
// provider if the location is held within a private repository.  Otherwise we
// can both probe GitHub's API as well as exploit the fact that it is a Git
// repository to retrieve information and deliver information as a Manifest
// format.
func NewGitHubProvider(ctx context.Context, path string, opts ...ManifestOption) (Provider, error) {
	var branch string

	repo, err := ghrepo.NewFromURL(path)
	if err != nil {
		return nil, err
	}

	provider := GitHubProvider{
		path:   path,
		repo:   repo,
		branch: branch,
		ctx:    ctx,
		mopts:  NewManifestOptions(opts...),
	}

	provider.client = github.NewClient(nil)
	if ghauth, ok := provider.mopts.auths[repo.RepoHost()]; ok {
		if !ghauth.VerifySSL {
			insecureClient := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}

			ctx = context.WithValue(
				ctx,
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

			provider.client, err = github.NewEnterpriseClient(
				endpoint.String(),
				endpoint.String(),
				oauth2Client,
			)
			if err != nil {
				return nil, err
			}
		} else {
			provider.client = github.NewClient(oauth2Client)
		}
	}

	return provider, nil
}

func (ghp GitHubProvider) Manifests() ([]*Manifest, error) {
	// Is this a wildcard? E.g. lib-*?
	if strings.HasSuffix(ghp.repo.RepoName(), "*") {
		return ghp.manifestsFromWildcard()
	}

	// Ultimately, since this is Git, we can use the GitProvider, and update the
	// path to the resource with a known location
	repo := ghp.path
	if len(ghp.branch) > 0 {
		repo += "@" + ghp.branch
	}
	manifest, err := gitProviderFromGitHub(ghp.ctx, repo, ghp.mopts.opts...)
	if err != nil {
		return nil, err
	}

	manifest.Provider = ghp

	return []*Manifest{manifest}, nil
}

func (ghp GitHubProvider) PullManifest(ctx context.Context, manifest *Manifest, popts ...pack.PullOption) error {
	if config.G[config.KraftKit](ctx).GitProtocol == "git" {
		return pullGit(ctx, manifest, popts...)
	}

	manifest.mopts = ghp.mopts

	if err := pullArchive(ctx, manifest, popts...); err != nil {
		log.G(ctx).Trace(err)
		return pullGit(ctx, manifest, popts...)
	}

	return nil
}

func (ghp GitHubProvider) String() string {
	return "github"
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
		log.G(ghp.ctx).WithFields(logrus.Fields{
			"page": opts.Page,
		}).Trace("querying GitHub API for repositories")
		more, resp, err := ghp.client.Repositories.ListByOrg(
			ghp.ctx,
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

			log.G(ghp.ctx).Infof("found via wildcard %s", *repo.CloneURL)
			repos = append(repos, repo)
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return ghp.appendRepositoriesToManifestInParallel(repos)
}

func (ghp GitHubProvider) appendRepositoriesToManifestInParallel(repos []*github.Repository) ([]*Manifest, error) {
	var manifests []*Manifest
	var errs []error
	var wg sync.WaitGroup
	var mu sync.RWMutex
	wg.Add(len(repos))

	for _, repo := range repos {
		go func(repo *github.Repository) {
			defer wg.Done()
			manifest, err := gitProviderFromGitHub(ghp.ctx, *repo.CloneURL, ghp.mopts.opts...)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}

			// Populate with GitHub API-centric information
			if repo.Description != nil {
				manifest.Description = *repo.Description
			}

			t, _, _, err := unikraft.GuessTypeNameVersion(*repo.Name)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}

			manifest.Type = t

			mu.Lock()
			manifests = append(manifests, manifest)
			mu.Unlock()
		}(repo)
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return manifests, nil
}

// gitProviderFromGitHub is a cheap internal hack since we know that GitProvider
// simply returns a slice containing exactly 1 Manifest so as to be complicit
// with the Provider interface.  We can exploit this knowledge by accepting an
// input GitHub repository and making the necessary known adjustments to the
// channel and version Resource attribute.
func gitProviderFromGitHub(ctx context.Context, repo string, opts ...ManifestOption) (*Manifest, error) {
	// Ultimately, since this is Git, we can use the GitProvider, and update the
	// path to the resource with a known location
	provider, err := NewGitProvider(ctx, repo, opts...)
	if err != nil {
		return nil, err
	}

	manifests, err := provider.Manifests()
	if err != nil {
		return nil, err
	}

	// Here's the "hack"
	manifest := manifests[0]

	ghr, err := ghrepo.NewFromURL(repo)
	if err != nil {
		return nil, err
	}

	for j, channel := range manifest.Channels {
		channel.Resource = ghrepo.BranchArchive(ghr, channel.Name)
		manifest.Channels[j] = channel
	}

	for j, version := range manifest.Versions {
		if version.Type == ManifestVersionGitSha {
			version.Resource = ghrepo.SHAArchive(ghr, version.Version)
		} else {
			version.Resource = ghrepo.TagArchive(ghr, version.Version)
		}

		manifest.Versions[j] = version
	}

	return manifest, nil
}

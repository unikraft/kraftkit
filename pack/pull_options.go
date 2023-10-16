// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pack

import (
	"kraftkit.sh/config"
)

// PullOptions contains the list of options which can be set for pulling a
// package.
type PullOptions struct {
	auths             map[string]config.AuthConfig
	calculateChecksum bool
	onProgress        func(progress float64)
	workdir           string
	useCache          bool
}

// Auths returns the set authentication config for a given domain or nil if the
// domain was not found.
func (ppo *PullOptions) Auths(domain string) *config.AuthConfig {
	if auth, ok := ppo.auths[domain]; ok {
		return &auth
	}

	return nil
}

// OnProgress calls (if set) an embedded progress function which can be used to
// update an external progress bar, for example.
func (ppo *PullOptions) OnProgress(progress float64) {
	if ppo.onProgress != nil {
		ppo.onProgress(progress)
	}
}

// Workdir returns the set working directory as part of the pull request
func (ppo *PullOptions) Workdir() string {
	return ppo.workdir
}

// CalculateChecksum returns whether the pull request should perform a check of
// the resource sum.
func (ppo *PullOptions) CalculateChecksum() bool {
	return ppo.calculateChecksum
}

// UseCache returns whether the pull should redirect to using a local cache if
// available.
func (ppo *PullOptions) UseCache() bool {
	return ppo.useCache
}

// PullOption is an option function which is used to modify PullOptions.
type PullOption func(opts *PullOptions) error

// NewPullOptions creates PullOptions
func NewPullOptions(opts ...PullOption) (*PullOptions, error) {
	options := &PullOptions{}

	for _, o := range opts {
		err := o(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

// WithPullAuthConfig sets the authentication config to use when pulling the
// package.
func WithPullAuthConfig(auth map[string]config.AuthConfig) PullOption {
	return func(opts *PullOptions) error {
		if opts.auths == nil {
			opts.auths = map[string]config.AuthConfig{}
		}

		for k, v := range auth {
			opts.auths[k] = v
		}

		return nil
	}
}

// WithPullProgressFunc set an optional progress function which is used as a
// callback during the transmission of the package and the host.
func WithPullProgressFunc(onProgress func(progress float64)) PullOption {
	return func(opts *PullOptions) error {
		opts.onProgress = onProgress
		return nil
	}
}

// WithPullWorkdir set the working directory context of the pull such that the
// resources of the package are placed there appropriately.
func WithPullWorkdir(workdir string) PullOption {
	return func(opts *PullOptions) error {
		opts.workdir = workdir
		return nil
	}
}

// WithPullChecksum to set whether to calculate and compare the checksum of the
// package.
func WithPullChecksum(calc bool) PullOption {
	return func(opts *PullOptions) error {
		opts.calculateChecksum = calc
		return nil
	}
}

// WithPullCache to set whether use cache if possible.
func WithPullCache(cache bool) PullOption {
	return func(opts *PullOptions) error {
		opts.useCache = cache
		return nil
	}
}

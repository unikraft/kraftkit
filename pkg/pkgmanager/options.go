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

package pkgmanager

import (
	"context"

	"go.unikraft.io/kit/config"
	"go.unikraft.io/kit/utils"

	"go.unikraft.io/kit/pkg/log"
)

// PackageManagerOptions contains configuration for the Package
type PackageManagerOptions struct {
	Log    log.Logger
	Config *config.Config

	dataDir func() string

	// ctx should contain all implementation-specific options, using
	// `context.WithValue`
	ctx context.Context

	// Store a list of the functions used to populate this struct, in case we wish
	// to call them again (used now in the umbrella manager).
	opts []PackageManagerOption
}

type PackageManagerOption func(opts *PackageManagerOptions) error

// NewPackageManagerOptions creates PackageManagerOptions
func NewPackageManagerOptions(ctx context.Context, opts ...PackageManagerOption) (*PackageManagerOptions, error) {
	options := &PackageManagerOptions{
		ctx:  ctx,
		opts: opts,
	}

	for _, o := range opts {
		err := o(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

// WithLogger defines the log.Logger
func WithLogger(l log.Logger) PackageManagerOption {
	return func(o *PackageManagerOptions) error {
		o.Log = l
		return nil
	}
}

// WithConfig provides access to global config
func WithConfig(c *config.Config) PackageManagerOption {
	return func(o *PackageManagerOptions) error {
		o.Config = c
		return nil
	}
}

type PullPackageOptions struct {
	withDependencies bool
	architectures    []string
	platforms        []string
	version          string
	allVersions      bool
}

type PullPackageOption func(opts *PullPackageOptions) error

// NewPullPackageOptions creates PullPackageOptions
func NewPullPackageOptions(opts ...PullPackageOption) (*PullPackageOptions, error) {
	options := &PullPackageOptions{}

	for _, o := range opts {
		err := o(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

func WithPullArchitecture(archs ...string) PullPackageOption {
	return func(opts *PullPackageOptions) error {
		for _, arch := range archs {
			if arch == "" {
				continue
			}

			if utils.Contains(opts.architectures, arch) {
				continue
			}

			opts.architectures = append(opts.architectures, archs...)
		}

		return nil
	}
}

func WithPullPlatform(plats ...string) PullPackageOption {
	return func(opts *PullPackageOptions) error {
		for _, plat := range plats {
			if plat == "" {
				continue
			}

			if utils.Contains(opts.platforms, plat) {
				continue
			}

			opts.platforms = append(opts.platforms, plats...)
		}

		return nil
	}
}

type SearchPackageOptions struct {
	architectures  []string
	platforms      []string
	version        string
	componenteType string
}

type SearchPackageOption func(opts *SearchPackageOptions) error

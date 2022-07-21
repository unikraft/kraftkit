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

package packmanager

import (
	"context"

	"go.unikraft.io/kit/config"
	"go.unikraft.io/kit/utils"

	"go.unikraft.io/kit/log"
	"go.unikraft.io/kit/unikraft"
)

// PackageManagerOptions contains configuration for the Package
type PackageManagerOptions struct {
	ConfigManager *config.ConfigManager
	Log           log.Logger

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
func WithConfigManager(cm *config.ConfigManager) PackageManagerOption {
	return func(o *PackageManagerOptions) error {
		o.ConfigManager = cm
		return nil
	}
}

type CatalogQuery struct {
	Types   []unikraft.ComponentType
	Name    string
	Version string
}

func NewCatalogQuery(s string) CatalogQuery {
	query := CatalogQuery{}
	return query
}

func (cq CatalogQuery) String() string {
	s := ""
	if len(cq.Types) == 1 {
		s += string(cq.Types[0]) + "-"
	} else if len(cq.Types) > 1 {
		var types []string
		for _, t := range cq.Types {
			types = append(types, string(t))
		}

		s += "{" + utils.ListJoinStr(types, ", ") + "}-"
	}

	if len(cq.Name) > 0 {
		s += cq.Name
	} else {
		s += "*"
	}

	if len(cq.Version) > 0 {
		s += ":" + cq.Version
	}

	return s
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file expect in compliance with the License.
package packmanager

import (
	"context"

	"kraftkit.sh/config"
	"kraftkit.sh/utils"

	"kraftkit.sh/log"
	"kraftkit.sh/unikraft"
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

func (pmopts *PackageManagerOptions) Context() context.Context {
	return pmopts.ctx
}

// CatalogQuery is the request structure with associated attributes which are
// used to search the package manager's catalog
type CatalogQuery struct {
	// Source specifies where the origin of the package
	Source string

	// Types specifies the associated list of possible types for the package
	Types []unikraft.ComponentType

	// Name specifies the name of the package
	Name string

	// Version specifies the version of the package
	Version string

	// NoCache forces the package manager to update values in-memory without
	// interacting with any underlying cache
	NoCache bool
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

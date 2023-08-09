// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package packmanager

import (
	"kraftkit.sh/config"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/utils"
)

// Query is the request structure with associated attributes which are used to
// search the package manager's catalog
type Query struct {
	// Source specifies where the origin of the package
	source string

	// Types specifies the associated list of possible types for the package
	types []unikraft.ComponentType

	// Name specifies the name of the package
	name string

	// Version specifies the version of the package
	version string

	// useCache forces the package manager to update values using what it has
	// locally.
	useCache bool

	// Auth contains required authentication for the query.
	auths map[string]config.AuthConfig

	// allTargets indicates that the query should package all targets together
	allTargets bool

	// Architecture specifies the architecture of the package
	architecture string

	// Platform specifies the platform of the package
	platform string

	// KConfig specifies the list of config options of the package
	kConfig []string
}

// Source specifies where the origin of the package
func (query *Query) Source() string {
	return query.source
}

// Types specifies the associated list of possible types for the package
func (query *Query) Types() []unikraft.ComponentType {
	return query.types
}

// Name specifies the name of the package
func (query *Query) Name() string {
	return query.name
}

// Version specifies the version of the package
func (query *Query) Version() string {
	return query.version
}

// AllTargets indicates that the query should package all targets together
func (query *Query) AllTargets() bool {
	return query.allTargets
}

// Architecture specifies the architecture of the package
func (query *Query) Architecture() string {
	return query.architecture
}

// Platform specifies the platform of the package
func (query *Query) Platform() string {
	return query.platform
}

// KConfig specifies the list of config options of the package
func (query *Query) KConfig() []string {
	return query.kConfig
}

// UseCache indicates whether the package manager should use any existing cache.
func (query *Query) UseCache() bool {
	return query.useCache
}

// Auth returns authentication configuration for a given domain or nil if the
// domain does not have (or require) any authentication.
func (query *Query) Auths() map[string]config.AuthConfig {
	return query.auths
}

func (query *Query) Fields() map[string]interface{} {
	return map[string]interface{}{
		"name":    query.name,
		"version": query.version,
		"source":  query.source,
		"types":   query.types,
		"cache":   query.useCache,
		"auth":    query.auths != nil,
	}
}

// QueryOption is a method-option which sets a specific query parameter.
type QueryOption func(*Query)

// NewQuery returns the finalized query given the provided options.
func NewQuery(qopts ...QueryOption) *Query {
	query := Query{}
	for _, qopt := range qopts {
		qopt(&query)
	}
	return &query
}

// WithArchitecture sets the query parameter for the architecture of the package.
func WithArchitecture(arch string) QueryOption {
	return func(query *Query) {
		query.architecture = arch
	}
}

// WithPlatform sets the query parameter for the platform of the package.
func WithPlatform(platform string) QueryOption {
	return func(query *Query) {
		query.platform = platform
	}
}

// WithKconfig sets the query parameter for the list of configuration options of the package.
func WithKConfig(kConfig []string) QueryOption {
	return func(query *Query) {
		query.kConfig = kConfig
	}
}

// WithSource sets the query parameter for the origin source of the package.
func WithSource(source string) QueryOption {
	return func(query *Query) {
		query.source = source
	}
}

// WithTypes sets the query parameter for the specific Unikraft types to search
// for.
func WithTypes(types ...unikraft.ComponentType) QueryOption {
	return func(query *Query) {
		query.types = types
	}
}

// WithName sets the query parameter for the name of the package.
func WithName(name string) QueryOption {
	return func(query *Query) {
		query.name = name
	}
}

// WithVersion sets the query parameter for the version of the package.
func WithVersion(version string) QueryOption {
	return func(query *Query) {
		query.version = version
	}
}

// WithCache sets whether to use local caching when making the query.
func WithCache(useCache bool) QueryOption {
	return func(query *Query) {
		query.useCache = useCache
	}
}

// WithAllTargets sets whether to package all targets together.
func WithAllTargets(allTargets bool) QueryOption {
	return func(query *Query) {
		query.allTargets = allTargets
	}
}

// WithAuthConfig sets the the required authorization for when making the query.
func WithAuthConfig(auths map[string]config.AuthConfig) QueryOption {
	return func(query *Query) {
		query.auths = auths
	}
}

func (cq Query) String() string {
	s := ""
	if len(cq.types) == 1 {
		s += string(cq.types[0]) + "-"
	} else if len(cq.types) > 1 {
		var types []string
		for _, t := range cq.types {
			types = append(types, string(t))
		}

		s += "{" + utils.ListJoinStr(types, ", ") + "}-"
	}

	if len(cq.name) > 0 {
		s += cq.name
	} else {
		s += "*"
	}

	if len(cq.version) > 0 {
		s += ":" + cq.version
	}

	return s
}

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

	// remote informs the package manager to update values from remote manifests.
	remote bool

	// local informs the package manager to update values from local manifests.
	local bool

	// Auth contains required authentication for the query.
	auths map[string]config.AuthConfig

	// Selects all the packages
	// (Currently, being used to prune all the packages on the host machine)
	all bool

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

// Remote indicates whether the package manager should use remote manifests
// when making its query.
func (query *Query) Remote() bool {
	return query.remote
}

// Local indicates whether the package manager should use local manifests
// when making its query.
func (query *Query) Local() bool {
	return query.local
}

// Auth returns authentication configuration for a given domain or nil if the
// domain does not have (or require) any authentication.
func (query *Query) Auths() map[string]config.AuthConfig {
	return query.auths
}

// All returns the value set for all.
func (query *Query) All() bool {
	return query.all
}

func (query *Query) Fields() map[string]interface{} {
	fields := map[string]interface{}{}

	if len(query.name) > 0 {
		fields["name"] = query.name
	}
	if len(query.version) > 0 {
		fields["version"] = query.version
	}
	if len(query.types) > 0 {
		fields["types"] = query.types
	}

	fields["remote"] = query.remote
	fields["local"] = query.local

	if len(query.auths) > 0 {
		fields["auths"] = "true"
	}
	if len(query.architecture) > 0 {
		fields["arch"] = query.architecture
	}
	if len(query.platform) > 0 {
		fields["plat"] = query.platform
	}
	if len(query.kConfig) > 0 {
		fields["kConfig"] = query.kConfig
	}

	return fields
}

// QueryOption is a method-option which sets a specific query parameter.
type QueryOption func(*Query)

// NewQuery returns the finalized query given the provided options.
func NewQuery(qopts ...QueryOption) *Query {
	query := Query{
		remote: false,
		local:  true,
	}
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

// WithRemote sets whether to use remote manifests when making the query.
func WithRemote(remote bool) QueryOption {
	return func(query *Query) {
		query.remote = remote
	}
}

// WithLocal sets whether to use local manifests when making the query.
func WithLocal(local bool) QueryOption {
	return func(query *Query) {
		query.local = local
	}
}

// WithAuthConfig sets the the required authorization for when making the query.
func WithAuthConfig(auths map[string]config.AuthConfig) QueryOption {
	return func(query *Query) {
		query.auths = auths
	}
}

func WithAll(all bool) QueryOption {
	return func(query *Query) {
		query.all = all
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

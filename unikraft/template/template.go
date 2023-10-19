// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package template implements the interface through which templates can
// be edited and configured.
package template

import (
	"context"

	"kraftkit.sh/kconfig"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/component"
)

// Template interfaces with the standard component in Unikraft
type Template interface {
	component.Component
}

// TemplateConfig is the configuration of a template. It is identical to
// the component configuration
type TemplateConfig struct {
	// name of the template.
	name string

	// version of the template.
	version string

	// source of the template (can be either remote or local, this attribute is
	// ultimately handled by the packmanager).
	source string

	// path to the template.
	path string

	// kconfig associated with the template.
	kconfig kconfig.KeyValueMap
}

// NewTemplateFromOptions creates a new template configuration
func NewTemplateFromOptions(opts ...TemplateOption) (Template, error) {
	tc := TemplateConfig{}

	for _, opt := range opts {
		if err := opt(&tc); err != nil {
			return nil, err
		}
	}

	return &tc, nil
}

// Name returns the name of the template
func (tc TemplateConfig) Name() string {
	return tc.name
}

func (tc TemplateConfig) String() string {
	return tc.name
}

// Source returns the source of the template
func (tc TemplateConfig) Source() string {
	return tc.source
}

// Version returns the version of the template
func (tc TemplateConfig) Version() string {
	return tc.version
}

// Type returns the type of the template
func (tc TemplateConfig) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

func (tc TemplateConfig) Path() string {
	return tc.path
}

// KConfigTree returns the path to the kconfig file of the template
func (tc TemplateConfig) KConfigTree(context.Context, ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, nil
}

func (tc TemplateConfig) KConfig() kconfig.KeyValueMap {
	return tc.kconfig
}

func (tc TemplateConfig) MarshalYAML() (interface{}, error) {
	return nil, nil
}

// PrintInfo prints information about the template
func (tc TemplateConfig) PrintInfo(ctx context.Context) string {
	return "not implemented: unikraft.template.TemplateConfig.PrintInfo"
}

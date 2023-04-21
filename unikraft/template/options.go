// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package template

// TemplateOption is a function that modifies a TemplateConfig.
type TemplateOption func(*TemplateConfig) error

// WithName sets the name of the template.
func WithName(name string) TemplateOption {
	return func(tc *TemplateConfig) error {
		tc.name = name
		return nil
	}
}

// WithVersion sets the version of the template.
func WithVersion(version string) TemplateOption {
	return func(tc *TemplateConfig) error {
		tc.version = version
		return nil
	}
}

// WithSource sets the source of the template.
func WithSource(source string) TemplateOption {
	return func(tc *TemplateConfig) error {
		tc.source = source
		return nil
	}
}

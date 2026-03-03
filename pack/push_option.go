// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package pack

import (
	"maps"

	"kraftkit.sh/config"
)

// PushOptions contains the list of options which can be set whilst pushing a
// package.
type PushOptions struct {
	onProgress func(progress float64)
	auths      map[string]config.AuthConfig
}

// PushOption is an option function which is used to modify PushOptions.
type PushOption func(*PushOptions) error

// NewPushOptions creates PushOptions
func NewPushOptions(opts ...PushOption) (*PushOptions, error) {
	options := &PushOptions{}

	for _, o := range opts {
		err := o(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

// OnProgress returns the embedded progress function which can be used to
// update an external progress bar.
func (ppo *PushOptions) OnProgress() func(float64) {
	return ppo.onProgress
}

// Auths returns the set authentication config for a given domain or nil if the
// domain was not found.
func (ppo *PushOptions) Auths() map[string]config.AuthConfig {
	return ppo.auths
}

// WithPushProgressFunc set an optional progress function which is used as a
// callback during the transmission of the package and the host.
func WithPushProgressFunc(onProgress func(progress float64)) PushOption {
	return func(opts *PushOptions) error {
		opts.onProgress = onProgress
		return nil
	}
}

// WithPushAuthConfig sets the authentication config to use when pushing the
// package.
func WithPushAuthConfig(auth map[string]config.AuthConfig) PushOption {
	return func(opts *PushOptions) error {
		if opts.auths == nil {
			opts.auths = map[string]config.AuthConfig{}
		}

		maps.Copy(opts.auths, auth)

		return nil
	}
}

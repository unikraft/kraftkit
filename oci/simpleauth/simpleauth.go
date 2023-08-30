// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

// Package simpleauth implements a basic pass-by-reference of credentials for
// the authn.Authenticator interface.
package simpleauth

import "github.com/google/go-containerregistry/pkg/authn"

// SimpleAuthenticator is used to handle looking up the already populated
// user configuration that is used when speaking with the remote registry.
type SimpleAuthenticator struct {
	Auth *authn.AuthConfig
}

// Authorization implements authn.Authenticator.
func (auth *SimpleAuthenticator) Authorization() (*authn.AuthConfig, error) {
	return auth.Auth, nil
}

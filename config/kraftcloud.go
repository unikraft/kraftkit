// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package config

import (
	"context"
	"fmt"
	"os"
)

// GetKraftCloudLogin is a utility method which retrieves credentials of a
// KraftCloud user from the given context which is populated with the
// current configuration.
func GetKraftCloudLoginFromContext(ctx context.Context) (*AuthConfig, error) {
	auth, ok := G[KraftKit](ctx).Auth["index.unikraft.io"]
	if !ok {
		return nil, fmt.Errorf("user not logged in to kraftcloud")
	}

	auth.Endpoint = "index.unikraft.io"

	// Attempt to fallback to environmental variables:
	if token := os.Getenv("KRAFTCLOUD_TOKEN"); token != "" {
		auth.Token = token
	} else {
		return nil, fmt.Errorf("could not determine kraftcloud user token: try setting `KRAFTCLOUD_TOKEN`")
	}

	if user := os.Getenv("KRAFTCLOUD_USER"); user != "" {
		auth.User = user
	}

	return &auth, nil
}

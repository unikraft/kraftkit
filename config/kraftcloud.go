// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package config

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// GetUnikraftCloudLogin is a utility method which retrieves credentials of a
// UnikraftCloud user from the given context returning it in AuthConfig format.
func GetUnikraftCloudAuthConfig(ctx context.Context, flagToken string) (*AuthConfig, error) {
	auth := AuthConfig{
		Endpoint:  "index.unikraft.io",
		VerifySSL: true,
	}

	if flagToken == "" {
		flagToken = os.Getenv("KRAFTCLOUD_TOKEN")
	}

	if flagToken == "" {
		flagToken = os.Getenv("KC_TOKEN")
	}

	if flagToken == "" {
		flagToken = os.Getenv("UNIKRAFTCLOUD_TOKEN")
	}

	if flagToken == "" {
		flagToken = os.Getenv("UKC_TOKEN")
	}

	// Prioritize environmental variables
	if flagToken != "" {
		data, err := base64.StdEncoding.DecodeString(flagToken)
		if err != nil {
			return nil, fmt.Errorf("could not decode token: %w", err)
		}

		split := strings.Split(string(data), ":")
		if len(split) != 2 {
			return nil, fmt.Errorf("invalid token format")
		}

		auth.User = split[0]
		auth.Token = split[1]

		if G[KraftKit](ctx).Auth == nil {
			authMap := map[string]AuthConfig{}
			authMap["index.unikraft.io"] = auth
			(*G[KraftKit](ctx)).Auth = authMap
		} else {
			G[KraftKit](ctx).Auth["index.unikraft.io"] = auth
		}

		// Fallback to local config
	} else if auth, ok := G[KraftKit](ctx).Auth["index.unikraft.io"]; ok {
		return &auth, nil
	} else {
		return nil, fmt.Errorf("could not determine unikraft cloud user token: try setting `UKC_TOKEN`")
	}

	return &auth, nil
}

// GetUnikraftCloudTokenAuthConfig is a utility method which returns the
// token of the UnikraftCloud user based on an AuthConfig.
func GetUnikraftCloudTokenAuthConfig(auth AuthConfig) string {
	return base64.StdEncoding.EncodeToString([]byte(auth.User + ":" + auth.Token))
}

// HydrateUnikraftCloudAuthInContext saturates the context with an additional
// cloud-specific information.
func HydrateUnikraftCloudAuthInContext(ctx context.Context) (context.Context, error) {
	auth, err := GetUnikraftCloudAuthConfig(ctx, "")
	if err != nil {
		return nil, err
	}

	if G[KraftKit](ctx).Auth == nil {
		G[KraftKit](ctx).Auth = make(map[string]AuthConfig)
	}

	G[KraftKit](ctx).Auth["index.unikraft.io"] = *auth

	return ctx, nil
}

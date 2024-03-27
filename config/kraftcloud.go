// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2023, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package config

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
)

// GetKraftCloudLogin is a utility method which retrieves credentials of a
// KraftCloud user from the given context returning it in AuthConfig format.
func GetKraftCloudAuthConfig(ctx context.Context, flagToken string) (*AuthConfig, error) {
	auth := AuthConfig{
		Endpoint:  "index.unikraft.io",
		VerifySSL: true,
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
	} else if localAuth, ok := G[KraftKit](ctx).Auth["index.unikraft.io"]; ok {
		auth = localAuth
	} else {
		return nil, fmt.Errorf("could not determine kraftcloud user token: try setting `KRAFTCLOUD_TOKEN`")
	}

	auth.User = maybePrependRobot(auth.User)

	return &auth, nil
}

// GetKraftCloudTokenAuthConfig is a utility method which returns the
// token of the KraftCloud user based on an AuthConfig.
func GetKraftCloudTokenAuthConfig(auth AuthConfig) string {
	return base64.StdEncoding.EncodeToString([]byte(auth.User + ":" + auth.Token))
}

// HydrateKraftCloudAuthInContext saturates the context with an additional
// KraftCloud-specific information.
func HydrateKraftCloudAuthInContext(ctx context.Context) (context.Context, error) {
	auth, err := GetKraftCloudAuthConfig(ctx, "")
	if err != nil {
		return nil, err
	}

	if G[KraftKit](ctx).Auth == nil {
		G[KraftKit](ctx).Auth = make(map[string]AuthConfig)
	}

	G[KraftKit](ctx).Auth["index.unikraft.io"] = *auth

	return ctx, nil
}

// NOTE(antoineco): this function exists because some tokens were issued
// without the Harbor `robot$` prefix in the username, causing requests to
// Harbor to fail (e.g. image pull/push).
func maybePrependRobot(user string) string {
	const (
		robotUserPrefix = "robot$"
		robotUserSuffix = ".users.kraftcloud"
	)
	if !strings.HasSuffix(user, robotUserSuffix) {
		return user
	}
	if strings.HasPrefix(user, robotUserPrefix) {
		return user
	}
	return robotUserPrefix + user
}

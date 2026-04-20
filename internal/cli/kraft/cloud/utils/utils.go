// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package utils

import (
	"context"
	"fmt"
	"slices"

	unikraftcloud "sdk.kraft.cloud"
)

func getPublicMetroCodes(ctx context.Context) ([]string, error) {
	client := unikraftcloud.NewMetrosClient()
	metros, err := client.List(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("could not list metros: %w", err)
	}

	candidates := make([]string, len(metros))
	for i, m := range metros {
		candidates[i] = m.Code
	}
	return candidates, nil
}

func getPublicMetroURLs(ctx context.Context) ([]string, error) {
	metroCodes, err := getPublicMetroCodes(ctx)
	if err != nil {
		return nil, err
	}

	urls := make([]string, len(metroCodes))
	for i, code := range metroCodes {
		urls[i] = fmt.Sprintf("https://api.%s.unikraft.cloud/v1", code)
	}

	return urls, nil
}

func getOldPublicMetroURLs(ctx context.Context) ([]string, error) {
	metroCodes, err := getPublicMetroCodes(ctx)
	if err != nil {
		return nil, err
	}

	urls := make([]string, len(metroCodes))
	for i, code := range metroCodes {
		urls[i] = fmt.Sprintf("https://api%s0.kraft.cloud/v1", code)
	}

	return urls, nil
}

// IsPublicMetro checks if the provided server URL corresponds to a public metro.
func IsPublicMetro(ctx context.Context, server string) (bool, error) {
	if server == "" {
		return false, nil
	}

	codes, err := getPublicMetroCodes(ctx)
	if err != nil {
		return false, err
	}

	urls, err := getPublicMetroURLs(ctx)
	if err != nil {
		return false, err
	}

	oldUrls, err := getOldPublicMetroURLs(ctx)
	if err != nil {
		return false, err
	}

	if slices.Contains(codes, server) {
		return true, nil
	}

	if slices.Contains(urls, server) {
		return true, nil
	}

	if slices.Contains(oldUrls, server) {
		return true, nil
	}

	return false, nil
}

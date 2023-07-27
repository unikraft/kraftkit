// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package network

import (
	"context"
	"errors"

	networkv1alpha1 "kraftkit.sh/api/network/v1alpha1"
)

// hostSupportedStrategies returns the map of known supported drivers for the
// given host.
func hostSupportedStrategies() map[string]*Strategy {
	return map[string]*Strategy{
		"bridge": {
			NewNetworkV1alpha1: func(ctx context.Context, opts ...any) (networkv1alpha1.NetworkService, error) {
				return nil, errors.New("network service is not supported on MacOS")
			},
		},
	}
}

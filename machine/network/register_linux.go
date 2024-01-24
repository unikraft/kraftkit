// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package network

import (
	"context"
	"path/filepath"

	zip "api.zip"

	networkv1alpha1 "kraftkit.sh/api/network/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/machine/network/bridge"
	"kraftkit.sh/store"
)

// hostSupportedStrategies returns the map of known supported drivers for the
// given host.
func hostSupportedStrategies() map[string]*Strategy {
	return map[string]*Strategy{
		"bridge": {
			NewNetworkV1alpha1: func(ctx context.Context, opts ...any) (networkv1alpha1.NetworkService, error) {
				service, err := bridge.NewNetworkServiceV1alpha1(ctx, opts...)
				if err != nil {
					return nil, err
				}

				embeddedStore, err := store.NewEmbeddedStore[networkv1alpha1.NetworkSpec, networkv1alpha1.NetworkStatus](
					filepath.Join(
						config.G[config.KraftKit](ctx).RuntimeDir,
						"networkv1alpha1",
					),
				)
				if err != nil {
					return nil, err
				}

				return networkv1alpha1.NewNetworkServiceHandler(
					ctx,
					service,
					zip.WithStore[networkv1alpha1.NetworkSpec, networkv1alpha1.NetworkStatus](embeddedStore, zip.StoreRehydrationSpecNil),
				)
			},
		},
	}
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package volume

import (
	"context"
	"path/filepath"

	zip "api.zip"

	volumev1alpha1 "kraftkit.sh/api/volume/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/machine/store"
	ninepfs "kraftkit.sh/machine/volume/9pfs"
)

// hostSupportedStrategies returns the map of known supported drivers for the
// given host.
func hostSupportedStrategies() map[string]*Strategy {
	return map[string]*Strategy{
		"9pfs": {
			IsCompatible: func(source string, _ kconfig.KeyValueMap) (bool, error) {
				// TODO(nderjung): For now, it is OK to return true because this is the
				// only supported driver.  In the future, we should a). check if the
				// provided source is a readable directory and b). check if the supplied
				// KConfig of the machine indicates that 9pfs is indeed part of the
				// build configuration.
				return true, nil
			},
			NewVolumeV1alpha1: func(ctx context.Context, opts ...any) (volumev1alpha1.VolumeService, error) {
				service, err := ninepfs.NewVolumeServiceV1alpha1(ctx, opts...)
				if err != nil {
					return nil, err
				}

				embeddedStore, err := store.NewEmbeddedStore[volumev1alpha1.VolumeSpec, volumev1alpha1.VolumeStatus](
					filepath.Join(
						config.G[config.KraftKit](ctx).RuntimeDir,
						"volumev1alpha1",
					),
				)
				if err != nil {
					return nil, err
				}

				return volumev1alpha1.NewVolumeServiceHandler(
					ctx,
					service,
					zip.WithStore[volumev1alpha1.VolumeSpec, volumev1alpha1.VolumeStatus](embeddedStore, zip.StoreRehydrationSpecNil),
				)
			},
		},
	}
}

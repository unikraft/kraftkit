// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package platform

import (
	"context"
	"path/filepath"

	zip "api.zip"
	machinev1alpha1 "kraftkit.sh/api/machine/v1alpha1"
	"kraftkit.sh/config"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/machine/firecracker"
	"kraftkit.sh/machine/store"
)

var firecrackerV1alpha1Driver = func(ctx context.Context, cfg *config.KraftKit, opts ...any) (machinev1alpha1.MachineService, error) {
	// cfg := cli.GetConfig()
	if set.NewStringSet("debug", "trace").Contains(cfg.Log.Level) {
		opts = append(opts, firecracker.WithDebug(true))
	}
	service, err := firecracker.NewMachineV1alpha1Service(ctx, opts...)
	if err != nil {
		return nil, err
	}

	embeddedStore, err := store.NewEmbeddedStore[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus](
		filepath.Join(
			cfg.RuntimeDir,
			"machinev1alpha1",
		),
	)
	if err != nil {
		return nil, err
	}

	return machinev1alpha1.NewMachineServiceHandler(
		ctx,
		service,
		cfg,
		zip.WithStore[machinev1alpha1.MachineSpec, machinev1alpha1.MachineStatus](embeddedStore, zip.StoreRehydrationSpecNil),
		zip.WithBefore(storePlatformFilter(PlatformFirecracker)),
	)
}

func unixVariantStrategies() map[Platform]*Strategy {
	// TODO(jake-ciolek): The firecracker driver has a dependency on github.com/containernetworking/plugins/pkg/ns via
	// github.com/firecracker-microvm/firecracker-go-sdk
	// Unfortunately, it doesn't support darwin.
	return map[Platform]*Strategy{
		PlatformFirecracker: {
			NewMachineV1alpha1: firecrackerV1alpha1Driver,
		},
	}
}

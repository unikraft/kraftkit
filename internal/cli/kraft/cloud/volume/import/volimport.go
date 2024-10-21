// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package vimport

import (
	"context"
	"fmt"
	"strconv"
	"time"

	ukcinstances "sdk.kraft.cloud/instances"
	ukcservices "sdk.kraft.cloud/services"
	ukcvolumes "sdk.kraft.cloud/volumes"
)

// volumeSanityCheck verifies that the given volume is suitable for import.
func volumeSanityCheck(ctx context.Context, cli ukcvolumes.VolumesService, volID string, dataSize int64) (volUUID string, volSize int64, err error) {
	getvolResp, err := cli.Get(ctx, volID)
	if err != nil {
		return "", -1, fmt.Errorf("getting volume details: %w", err)
	}
	vol, err := getvolResp.FirstOrErr()
	if err != nil {
		return "", -1, fmt.Errorf("getting volume details: %w", err)
	}

	if volSize = int64(vol.SizeMB) * 1024 * 1024; dataSize >= volSize {
		return "", -1, fmt.Errorf("volume too small for input data (%d/%d)", dataSize, volSize)
	}

	return vol.UUID, volSize, nil
}

// runVolimport spawns a volume data import instance with the given volume attached.
func runVolimport(ctx context.Context, cli ukcinstances.InstancesService, image, volUUID, authStr string, timeoutS uint64) (instID, fqdn string, err error) {
	args := []string{
		"-p", strconv.FormatUint(uint64(volimportPort), 10),
		"-a", authStr,
		"-t", strconv.FormatUint(timeoutS, 10),
	}

	crinstResp, err := cli.Create(ctx, ukcinstances.CreateRequest{
		Image:    image,
		MemoryMB: ptr(128),
		Args:     args,
		ServiceGroup: &ukcinstances.CreateRequestServiceGroup{
			Services: []ukcservices.CreateRequestService{{
				Port:            int(volimportPort),
				DestinationPort: ptr(int(volimportPort)),
				Handlers:        []ukcservices.Handler{ukcservices.HandlerTLS},
			}},
		},
		Volumes: []ukcinstances.CreateRequestVolume{{
			UUID: &volUUID,
			At:   ptr("/"),
		}},
		Autostart:     ptr(true),
		WaitTimeoutMs: ptr(int((3 * time.Second).Milliseconds())),
		Features:      []ukcinstances.Feature{ukcinstances.FeatureDeleteOnStop},
	})
	if err != nil {
		return "", "", fmt.Errorf("creating volume data import instance: %w", err)
	}
	inst, err := crinstResp.FirstOrErr()
	if err != nil {
		if inst != nil && inst.Name != "" {
			// Delete the instance if it was created but failed to start
			crdelResp, err := cli.Delete(ctx, inst.UUID)
			if err != nil {
				return "", "", fmt.Errorf("deleting volume data import instance on fail: %w", err)
			}

			if _, err = crdelResp.FirstOrErr(); err != nil {
				return "", "", fmt.Errorf("deleting volume data import instance on fail: %w", err)
			}
		}
		return "", "", fmt.Errorf("creating volume data import instance: %w", err)
	}

	return inst.UUID, inst.ServiceGroup.Domains[0].FQDN, nil
}

func ptr[T comparable](v T) *T { return &v }

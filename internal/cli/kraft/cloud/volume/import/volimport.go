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

	kcinstances "sdk.kraft.cloud/instances"
	kcservices "sdk.kraft.cloud/services"
	kcvolumes "sdk.kraft.cloud/volumes"

	"kraftkit.sh/internal/retrytimeout"
)

// volumeSanityCheck verifies that the given volume is suitable for import.
func volumeSanityCheck(ctx context.Context, cli kcvolumes.VolumesService, volID string, dataSize int64) (volUUID string, volSize int64, err error) {
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
func runVolimport(ctx context.Context, cli kcinstances.InstancesService, image, volUUID, authStr string, timeoutS uint64) (instID, fqdn string, err error) {
	args := []string{
		"-p", strconv.FormatUint(uint64(volimportPort), 10),
		"-a", authStr,
	}

	if timeoutS > 0 {
		// Note(craciunoiuc): Add a 10-second buffer to the timeout.
		// This is to allow the client to close the connection first.
		// Otherwise there is a chance that the volume becomes corrupted.
		args = append(args, "-t", strconv.FormatUint(timeoutS+10, 10))
	}

	crinstResp, err := cli.Create(ctx, kcinstances.CreateRequest{
		Image:    image,
		MemoryMB: ptr(128),
		Args:     args,
		ServiceGroup: &kcinstances.CreateRequestServiceGroup{
			Services: []kcservices.CreateRequestService{{
				Port:            int(volimportPort),
				DestinationPort: ptr(int(volimportPort)),
				Handlers:        []kcservices.Handler{kcservices.HandlerTLS},
			}},
		},
		Volumes: []kcinstances.CreateRequestVolume{{
			UUID: &volUUID,
			At:   ptr("/"),
		}},
		Autostart:     ptr(true),
		WaitTimeoutMs: ptr(int((3 * time.Second).Milliseconds())),
		Features:      []kcinstances.Feature{kcinstances.FeatureDeleteOnStop},
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

// terminateVolimport deletes the volume data import instance once it has
// reached the "stopped" state.
func terminateVolimport(ctx context.Context, icli kcinstances.InstancesService, instID string) error {
	err := retrytimeout.RetryTimeout(3*time.Second, func() (retErr error) {
		defer func() {
			if retErr != nil {
				time.Sleep(time.Millisecond * 100)
			}
		}()
		getinstResp, err := icli.Get(ctx, instID)
		if err != nil {
			return fmt.Errorf("getting instance status: %w", err)
		}
		inst, err := getinstResp.FirstOrErr()
		if err != nil {
			return fmt.Errorf("getting instance status: %w", err)
		}
		if inst.State != "stopped" {
			return fmt.Errorf("instance has not yet stopped (state: %s)", inst.State)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("waiting for volume data import instance '%s' to stop: %w", instID, err)
	}

	delinstResp, err := icli.Delete(ctx, instID)
	if err != nil {
		return fmt.Errorf("deleting volume data import instance '%s': %w", instID, err)
	}
	if _, err = delinstResp.FirstOrErr(); err != nil {
		return fmt.Errorf("deleting volume data import instance '%s': %w", instID, err)
	}
	return nil
}

func ptr[T comparable](v T) *T { return &v }

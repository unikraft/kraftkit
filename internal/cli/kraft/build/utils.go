// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package build

import (
	"context"
	"fmt"
	"path/filepath"

	"kraftkit.sh/initrd"
	"kraftkit.sh/unikraft"
)

// buildRootfs generates a rootfs based on the provided
func (opts *Build) buildRootfs(ctx context.Context) error {
	if opts.Rootfs == "" {
		return nil
	}

	ramfs, err := initrd.New(ctx, opts.Rootfs,
		initrd.WithOutput(filepath.Join(opts.workdir, unikraft.BuildDir, initrd.DefaultInitramfsFileName)),
		initrd.WithCacheDir(filepath.Join(opts.workdir, unikraft.VendorDir, "rootfs-cache")),
	)
	if err != nil {
		return fmt.Errorf("could not prepare initramfs: %w", err)
	}

	if _, err := ramfs.Build(ctx); err != nil {
		return err
	}

	return nil
}

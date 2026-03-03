// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package initrd

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"kraftkit.sh/archive"
	"kraftkit.sh/fs/cpio"
	"kraftkit.sh/fs/erofs"
)

type tarball struct {
	opts InitrdOptions
	path string
}

func NewFromTarball(_ context.Context, tb string, opts ...InitrdOption) (Initrd, error) {
	rootfs := tarball{
		opts: InitrdOptions{
			fsType: FsTypeCpio,
		},
		path: tb,
	}

	for _, opt := range opts {
		if err := opt(&rootfs.opts); err != nil {
			return nil, err
		}
	}

	if !path.IsAbs(tb) {
		rootfs.path = filepath.Join(rootfs.opts.workdir, tb)
	}

	if tarOk, _ := archive.IsTarGz(rootfs.path); !tarOk {
		if tarGzOk, _ := archive.IsTar(rootfs.path); !tarGzOk {
			return nil, fmt.Errorf("supplied path is not a tarball: %s", rootfs.path)
		}
	}

	return &rootfs, nil
}

// Name implements Initrd.
func (initrd *tarball) Name() string {
	return "tarball"
}

// Build implements Initrd.
func (initrd *tarball) Build(ctx context.Context) (string, error) {
	if initrd.opts.output == "" {
		fi, err := os.CreateTemp("", "")
		if err != nil {
			return "", err
		}

		initrd.opts.output = fi.Name()
	}

	if err := os.MkdirAll(filepath.Dir(initrd.opts.output), 0o755); err != nil {
		return "", fmt.Errorf("could not create output directory: %w", err)
	}

	switch initrd.opts.fsType {
	case FsTypeUnknown:
		return "", fmt.Errorf("cannot build initrd from tarball with unknown filesystem type")
	case FsTypeFile:
		return initrd.opts.output, copyFile(initrd.path, initrd.opts.output)
	case FsTypeErofs:
		return initrd.opts.output, erofs.CreateFS(ctx, initrd.opts.output, initrd.path,
			erofs.WithAllRoot(!initrd.opts.keepOwners),
		)
	case FsTypeCpio:
		err := cpio.CreateFS(ctx, initrd.opts.output, initrd.path,
			cpio.WithAllRoot(!initrd.opts.keepOwners),
		)
		if err != nil {
			return "", fmt.Errorf("could not create CPIO archive: %w", err)
		}
		if initrd.opts.compress {
			if err := compressFiles(initrd.opts.output, initrd.opts.output); err != nil {
				return "", fmt.Errorf("could not compress files: %w", err)
			}
		}

		return initrd.opts.output, nil
	default:
		return "", fmt.Errorf("unknown filesystem type %s", initrd.opts.fsType)
	}
}

// Options implements Initrd.
func (initrd *tarball) Options() InitrdOptions {
	return initrd.opts
}

// Env implements Initrd.
func (initrd *tarball) Env() []string {
	return nil
}

// Args implements Initrd.
func (initrd *tarball) Args() []string {
	return nil
}

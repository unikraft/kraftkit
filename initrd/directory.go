// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"kraftkit.sh/fs/cpio"
	"kraftkit.sh/fs/erofs"
)

type directory struct {
	opts InitrdOptions
	path string
}

// NewFromDirectory returns an instantiated Initrd interface which is is able to
// serialize a rootfs from a given directory.
func NewFromDirectory(_ context.Context, dir string, opts ...InitrdOption) (Initrd, error) {
	dir = strings.TrimRight(dir, string(filepath.Separator))
	rootfs := directory{
		opts: InitrdOptions{
			fsType: FsTypeCpio,
		},
		path: dir,
	}

	for _, opt := range opts {
		if err := opt(&rootfs.opts); err != nil {
			return nil, err
		}
	}

	if !path.IsAbs(dir) {
		rootfs.path = filepath.Join(rootfs.opts.workdir, dir)
	}

	fi, err := os.Stat(rootfs.path)
	if err != nil && os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", rootfs.path)
	} else if err != nil {
		return nil, fmt.Errorf("could not check path: %w", err)
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("supplied path is not a directory: %s", rootfs.path)
	}

	return &rootfs, nil
}

// Build implements Initrd.
func (initrd *directory) Name() string {
	return "directory"
}

// Build implements Initrd.
func (initrd *directory) Build(ctx context.Context) (string, error) {
	if initrd.opts.output == "" {
		fi, err := os.CreateTemp("", "")
		if err != nil {
			return "", fmt.Errorf("could not make temporary file: %w", err)
		}

		initrd.opts.output = fi.Name()
		err = fi.Close()
		if err != nil {
			return "", fmt.Errorf("could not close temporary file: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(initrd.opts.output), 0o755); err != nil {
		return "", fmt.Errorf("could not create output directory: %w", err)
	}

	switch initrd.opts.fsType {
	case FsTypeFile:
		return "", fmt.Errorf("cannot build initrd from directory with file output type")
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
func (initrd *directory) Options() InitrdOptions {
	return initrd.opts
}

// Env implements Initrd.
func (initrd *directory) Env() []string {
	return nil
}

// Args implements Initrd.
func (initrd *directory) Args() []string {
	return nil
}

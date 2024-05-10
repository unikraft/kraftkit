// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cavaliergopher/cpio"
)

type directory struct {
	opts  InitrdOptions
	path  string
	files []string
}

// NewFromDirectory returns an instantiated Initrd interface which is is able to
// serialize a rootfs from a given directory.
func NewFromDirectory(_ context.Context, path string, opts ...InitrdOption) (Initrd, error) {
	path = strings.TrimRight(path, string(filepath.Separator))
	rootfs := directory{
		opts: InitrdOptions{},
		path: path,
	}

	for _, opt := range opts {
		if err := opt(&rootfs.opts); err != nil {
			return nil, err
		}
	}

	fi, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", path)
	} else if err != nil {
		return nil, fmt.Errorf("could not check path: %w", err)
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("supplied path is not a directory: %s", path)
	}

	return &rootfs, nil
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

	f, err := os.OpenFile(initrd.opts.output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("could not open initramfs file: %w", err)
	}

	defer f.Close()

	writer := cpio.NewWriter(f)
	defer writer.Close()

	if err := walkFiles(ctx, initrd.path, writer, &initrd.files); err != nil {
		return "", fmt.Errorf("could not walk output path: %w", err)
	}

	if initrd.opts.compress {
		if err := compressFiles(initrd.opts.output, writer, f); err != nil {
			return "", fmt.Errorf("could not compress files: %w", err)
		}
	}

	return initrd.opts.output, nil
}

// Files implements Initrd.
func (initrd *directory) Files() []string {
	return initrd.files
}

// Env implements Initrd.
func (initrd *directory) Env() []string {
	return nil
}

// Args implements Initrd.
func (initrd *directory) Args() []string {
	return nil
}

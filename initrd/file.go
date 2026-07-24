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

	"kraftkit.sh/fs/cpio"
	"kraftkit.sh/fs/erofs"
	"kraftkit.sh/fsutils"

	kraftfilev07 "unikraft.com/x/kraftfile"
)

type file struct {
	opts InitrdOptions
	path string
}

// NewFromFile accepts an input file which already represents a CPIO archive and
// is provided as a mechanism for satisfying the Initrd interface.
func NewFromFile(_ context.Context, path string, opts ...InitrdOption) (Initrd, error) {
	initrd := file{
		opts: InitrdOptions{
			fsType: FsTypeCpio,
		},
		path: path,
	}

	for _, opt := range opts {
		if err := opt(&initrd.opts); err != nil {
			return nil, err
		}
	}

	if !filepath.IsAbs(initrd.path) {
		initrd.path = filepath.Join(initrd.opts.workdir, initrd.path)
	}

	stat, err := os.Stat(initrd.path)
	if err != nil {
		return nil, err
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("path %s is a directory, not a file", initrd.path)
	}

	return &initrd, nil
}

// Name implements Initrd.
func (initrd *file) Name() string {
	return "file"
}

// Build implements Initrd.
func (initrd *file) Build(ctx context.Context) (string, error) {
	if initrd.opts.fsType == FsTypeUnknown {
		initrd.opts.fsType = detectFsType(initrd.path)
		if initrd.opts.fsType == FsTypeUnknown {
			return "", fmt.Errorf("could not detect filesystem type of input file, please specify it explicitly via the --fs-type flag")
		}
	}

	absSrc, err := filepath.Abs(filepath.Clean(initrd.path))
	if err != nil {
		return "", fmt.Errorf("getting absolute path of source: %w", err)
	}

	if initrd.opts.output != "" {
		absDest, err := filepath.Abs(filepath.Clean(initrd.opts.output))
		if err != nil {
			return "", fmt.Errorf("getting absolute path of destination: %w", err)
		}

		if absDest == absSrc {
			if isMatchingFsType(absSrc, initrd.opts.fsType) {
				return initrd.path, nil
			}
			return "", fmt.Errorf("output path %q is the same as the source path %q; refusing to overwrite in-place (would corrupt the input)", absDest, absSrc)
		}
	}

	if initrd.opts.output == "" {
		if isMatchingFsType(absSrc, initrd.opts.fsType) {
			return initrd.path, nil
		}

		fi, err := os.CreateTemp("", "")
		if err != nil {
			return "", fmt.Errorf("could not make temporary file: %w", err)
		}
		initrd.opts.output = fi.Name()
		if err := fi.Close(); err != nil {
			return "", fmt.Errorf("could not close temporary file: %w", err)
		}
	}

	switch initrd.opts.fsType {
	case FsTypeFile:
		return initrd.opts.output, copyFile(initrd.path, initrd.opts.output)
	case FsTypeErofs:
		return initrd.opts.output, erofs.CreateFS(ctx, initrd.opts.output, initrd.path,
			erofs.WithAllRoot(!initrd.opts.keepOwners),
		)
	case FsTypeCpio:
		return initrd.opts.output, cpio.CreateFS(ctx, initrd.opts.output, initrd.path,
			cpio.WithAllRoot(!initrd.opts.keepOwners),
		)
	default:
		return "", fmt.Errorf("unknown filesystem type %s", initrd.opts.fsType)
	}
}

func isMatchingFsType(source string, target kraftfilev07.FsType) bool {
	switch target {
	case FsTypeCpio:
		return fsutils.IsCpioFile(source)
	case FsTypeErofs:
		return fsutils.IsErofsFile(source)
	default:
		return false
	}
}

// detectFsType attempts to convert from the 'unknown' filesystem type to one
// of the known types like 'cpio'/'erofs'/'file'.
func detectFsType(source string) kraftfilev07.FsType {
	switch {
	case fsutils.IsErofsFile(source):
		return FsTypeErofs
	case fsutils.IsCpioFile(source):
		return FsTypeCpio
	case fsutils.IsTarFile(source),
		fsutils.IsTarGzFile(source),
		fsutils.IsRegularFile(source):
		return FsTypeFile
	default:
		return FsTypeUnknown
	}
}

// Options implements Initrd.
func (initrd *file) Options() InitrdOptions {
	return initrd.opts
}

// Env implements Initrd.
func (initrd *file) Env() []string {
	return nil
}

// Args implements Initrd.
func (initrd *file) Args() []string {
	return nil
}

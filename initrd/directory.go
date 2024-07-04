// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cavaliergopher/cpio"
	"kraftkit.sh/log"
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
		opts: InitrdOptions{},
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

	f, err := os.OpenFile(initrd.opts.output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("could not open initramfs file: %w", err)
	}

	defer f.Close()

	writer := cpio.NewWriter(f)
	defer writer.Close()
	defer func() {
		if err := f.Sync(); err != nil {
			log.G(ctx).Errorf("syncing cpio archive failed: %s", err)
		}
	}()

	// Recursively walk the output directory on successful build and serialize to
	// the output
	if err := filepath.WalkDir(initrd.path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("received error before parsing path: %w", err)
		}

		internal := strings.TrimPrefix(path, filepath.Clean(initrd.path))
		if internal == "" {
			return nil // Do not archive empty paths
		}
		internal = "." + filepath.ToSlash(internal)

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("could not get directory entry info: %w", err)
		}

		if d.Type().IsDir() {
			header := &cpio.Header{
				Name:    internal,
				Mode:    cpio.FileMode(info.Mode().Perm()) | cpio.TypeDir,
				ModTime: info.ModTime(),
				Size:    0, // Directories have size 0 in cpio
			}

			// Populate platform specific information
			populateCPIO(info, header)

			if err := writer.WriteHeader(header); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}
			return nil
		}

		log.G(ctx).
			WithField("file", internal).
			Trace("archiving")

		var data []byte
		targetLink := ""
		if info.Mode()&os.ModeSymlink != 0 {
			targetLink, err = os.Readlink(path)
			data = []byte(targetLink)
		} else if d.Type().IsRegular() {
			data, err = os.ReadFile(path)
		} else {
			log.G(ctx).Warnf("unsupported file: %s", path)
			return nil
		}
		if err != nil {
			return fmt.Errorf("could not read file: %w", err)
		}

		header := &cpio.Header{
			Name:    internal,
			Mode:    cpio.FileMode(info.Mode().Perm()),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}

		// Populate platform specific information
		populateCPIO(info, header)

		switch {
		case info.Mode().IsRegular():
			header.Mode |= cpio.TypeReg

		case info.Mode()&fs.ModeSymlink != 0:
			header.Mode |= cpio.TypeSymlink
			header.Linkname = targetLink

		case header.Links > 0:
			header.Size = 0
		}

		if err := writer.WriteHeader(header); err != nil {
			return fmt.Errorf("writing cpio header for %q: %w", internal, err)
		}

		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("could not write CPIO data for %s: %w", internal, err)
		}

		return nil
	}); err != nil {
		return "", fmt.Errorf("could not walk output path: %w", err)
	}

	if initrd.opts.compress {
		if err := compressFiles(initrd.opts.output, writer, f); err != nil {
			return "", fmt.Errorf("could not compress files: %w", err)
		}
	}

	return initrd.opts.output, nil
}

// Env implements Initrd.
func (initrd *directory) Env() []string {
	return nil
}

// Args implements Initrd.
func (initrd *directory) Args() []string {
	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package archive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"kraftkit.sh/log"
)

// bufPool is a pool of byte buffers that can be reused for copying content
// between files.
var bufPool = sync.Pool{
	New: func() interface{} {
		// The buffer size should be larger than or equal to 128 KiB for performance
		// considerations.  We choose 1 MiB here so there will be less disk I/O.
		buffer := make([]byte, 1<<20) // buffer size = 1 MiB
		return &buffer
	},
}

// TarFileWriter creates a tarball of a src to a dst using the provided tw
// tarball writer.
func TarFileWriter(ctx context.Context, src, dst string, tw *tar.Writer, opts ...ArchiveOption) error {
	dst = filepath.ToSlash(dst)

	if dst == "" {
		return fmt.Errorf("cannot tar file with no specified destination")
	} else if dst[0] == filepath.Separator {
		dst = dst[1:]
	}
	if strings.HasSuffix(dst, string(filepath.Separator)) {
		return fmt.Errorf("attempting to use TarFileWriter with directory")
	}

	aopts := ArchiveOptions{}
	for _, opt := range opts {
		if err := opt(&aopts); err != nil {
			return err
		}
	}

	fi, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("fail to stat %s: %v", src, err)
	}

	var link string
	mode := fi.Mode()
	if mode&os.ModeSymlink != 0 {
		if link, err = os.Readlink(src); err != nil {
			return err
		}
	}

	header, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}

	header.Name = dst
	header.Uid = 0
	header.Gid = 0
	header.Uname = ""
	header.Gname = ""
	header.Size = fi.Size()

	if aopts.stripTimes {
		header.ModTime = time.Time{}
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}
	}

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("tar: %w", err)
	}

	if mode.IsRegular() {
		log.G(ctx).WithFields(logrus.Fields{
			"dst": dst,
			"src": src,
		}).Trace("archive: tarring")

		fp, err := os.Open(src)
		if err != nil {
			return fmt.Errorf("fail to open file %s: %v", src, err)
		}

		buf := bufPool.Get().(*[]byte)
		defer bufPool.Put(buf)

		if _, err := io.CopyBuffer(tw, fp, *buf); err != nil {
			return fmt.Errorf("failed to copy to %s: %w", src, err)
		}

		if err := fp.Close(); err != nil {
			return err
		}
	}

	return nil
}

// TarFileTo accepts an input file `src` and places it exactly with the desired
// location `dst` inside the resulting artifact which is located at `out`.
func TarFileTo(ctx context.Context, src, dst, out string, opts ...ArchiveOption) error {
	aopts := ArchiveOptions{}
	for _, opt := range opts {
		if err := opt(&aopts); err != nil {
			return err
		}
	}

	fp, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("could not create tarball file: %s: %v", out, err)
	}

	var tw *tar.Writer
	var gzw *gzip.Writer

	if aopts.gzip {
		gzw = gzip.NewWriter(fp)
		tw = tar.NewWriter(gzw)
	} else {
		tw = tar.NewWriter(fp)
	}

	if err := TarFileWriter(ctx, src, dst, tw, opts...); err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}

	if aopts.gzip {
		if err := gzw.Close(); err != nil {
			return err
		}
	}

	if err := fp.Sync(); err != nil {
		return err
	}

	return fp.Close()
}

// TarFile creates a tarball from a given `src` file to the provided `out` file.
func TarFile(ctx context.Context, src, prefix, out string, opts ...ArchiveOption) error {
	fp, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("could not create tarball file: %s: %v", out, err)
	}

	tw := tar.NewWriter(fp)
	if err := TarFileWriter(ctx, src, filepath.Join(prefix, filepath.Base(src)), tw, opts...); err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}

	return fp.Close()
}

// TarDir creates a tarball of a given `root` into the provided `out` path.
func TarDir(ctx context.Context, root, prefix, out string, opts ...ArchiveOption) error {
	fp, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("could not create tarball file: %s: %v", out, err)
	}

	tw := tar.NewWriter(fp)
	if err := TarDirWriter(ctx, root, prefix, tw, opts...); err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}

	return fp.Close()
}

// TarDirWriter makes a tarball of a given `root` directory into the provided
// tarball writer `tw`.
func TarDirWriter(ctx context.Context, root, prefix string, tw *tar.Writer, opts ...ArchiveOption) error {
	return filepath.Walk(root, func(path string, _ os.FileInfo, err error) (returnErr error) {
		if err != nil {
			return err
		}

		dst, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		dst = filepath.ToSlash(filepath.Join(prefix, dst))

		return TarFileWriter(ctx, filepath.Join(root, path), dst, tw, opts...)
	})
}

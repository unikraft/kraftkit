// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2024, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cavaliergopher/cpio"
	"kraftkit.sh/log"
)

func walkFiles(ctx context.Context, outputDir string, writer *cpio.Writer, files *[]string) error {
	// Recursively walk the output directory on successful build and serialize to
	// the output
	return filepath.WalkDir(outputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("received error before parsing path: %w", err)
		}

		internal := strings.TrimPrefix(path, filepath.Clean(outputDir))
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

		*files = append(*files, internal)

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
		}

		if err := writer.WriteHeader(header); err != nil {
			return fmt.Errorf("writing cpio header for %q: %w", internal, err)
		}

		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("could not write CPIO data for %s: %w", internal, err)
		}

		return nil
	})
}

func compressFiles(output string, writer *cpio.Writer, reader *os.File) error {
	err := writer.Close()
	if err != nil {
		return fmt.Errorf("could not close CPIO writer: %w", err)
	}

	_, err = reader.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("could not seek to start of file: %w", err)
	}

	fw, err := os.OpenFile(output+".gz", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("could not open initramfs file: %w", err)
	}

	gw := gzip.NewWriter(fw)

	if _, err := io.Copy(gw, reader); err != nil {
		return fmt.Errorf("could not compress initramfs file: %w", err)
	}

	err = gw.Close()
	if err != nil {
		return fmt.Errorf("could not close gzip writer: %w", err)
	}

	err = fw.Close()
	if err != nil {
		return fmt.Errorf("could not close compressed initramfs file: %w", err)
	}

	if err := os.Remove(output); err != nil {
		return fmt.Errorf("could not remove uncompressed initramfs: %w", err)
	}

	if err := os.Rename(output+".gz", output); err != nil {
		return fmt.Errorf("could not rename compressed initramfs: %w", err)
	}

	return nil
}

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package erofs

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/unikraft/go-archivefs/erofs"

	"kraftkit.sh/fsutils"
	"kraftkit.sh/log"
)

type createOptions struct{}

func CreateFS(ctx context.Context, output string, source string, opts ...ErofsCreateOption) error {
	c := &createOptions{}

	// Open writer for the output file
	writer, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("could not open output file: %w", err)
	}
	defer writer.Close()

	switch {
	case fsutils.IsOciArchive(source):
		if err := c.CreateFSFromOCIImage(ctx, writer, source, opts...); err != nil {
			return fmt.Errorf("could not create EroFS archive from OCI image: %w", err)
		}
	case fsutils.IsErofsFile(source):
		if err := c.CreateFSFromErofs(ctx, writer, source, opts...); err != nil {
			return fmt.Errorf("could not create EroFS archive from CPIO file: %w", err)
		}
	case fsutils.IsCpioFile(source):
		return fmt.Errorf("creating EroFS from CPIO files is not currently supported")
	case fsutils.IsTarFile(source),
		fsutils.IsTarGzFile(source):
		if err := c.CreateFSFromTarFile(ctx, writer, source, opts...); err != nil {
			return fmt.Errorf("could not create EroFS archive from tar file: %w", err)
		}
	case fsutils.IsDirectory(source):
		if err := c.CreateFSFromDirectory(ctx, writer, source, opts...); err != nil {
			return fmt.Errorf("could not create EroFS archive from directory: %w", err)
		}
	default:
		return fmt.Errorf("unsupported source type: %s", source)
	}

	return nil
}

// CreateFSFromOCIImage creates an EroFS filesystem from an OCI image.
func (c *createOptions) CreateFSFromOCIImage(ctx context.Context, writer *os.File, source string, opts ...ErofsCreateOption) error {
	source, fInfoMap, err := fsutils.UnpackOCIImageToDirectory(ctx, source)
	if err != nil {
		return fmt.Errorf("could not unpack OCI file: %w", err)
	}
	defer os.RemoveAll(source)

	opts = append(opts, withFileInfoMap(fInfoMap))

	if err := c.CreateFSFromDirectory(ctx, writer, source, opts...); err != nil {
		return fmt.Errorf("could not create EroFS archive from directory: %w", err)
	}

	return nil
}

// CreateFSFromDirectory creates an EroFS filesystem from a directory.
func (c *createOptions) CreateFSFromDirectory(ctx context.Context, writer *os.File, source string, opts ...ErofsCreateOption) error {
	log.G(ctx).Info("creating EroFS archive")
	archivefsOpts, err := toArchivefsOptions(opts)
	if err != nil {
		return err
	}

	return erofs.Create(io.WriterAt(writer), os.DirFS(source), archivefsOpts...)
}

// CreateFSFromTarFile creates an EroFS filesystem from a tar file.
func (c *createOptions) CreateFSFromTarFile(ctx context.Context, writer *os.File, source string, opts ...ErofsCreateOption) error {
	source, fInfoMap, err := fsutils.UnpackTarFileToDirectory(ctx, source)
	if err != nil {
		return fmt.Errorf("could not unpack tar file: %w", err)
	}
	defer os.RemoveAll(source)

	opts = append(opts, withFileInfoMap(fInfoMap))

	if err := c.CreateFSFromDirectory(ctx, writer, source, opts...); err != nil {
		return fmt.Errorf("could not create EroFS archive from directory: %w", err)
	}

	return nil
}

// CreateFSFromErofs creates an EroFS filesystem from an existing EroFS file.
func (c *createOptions) CreateFSFromErofs(ctx context.Context, writer *os.File, source string, opts ...ErofsCreateOption) error {
	// Open and copy all contents from 'source' to the writer
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("could not open EroFS source file: %w", err)
	}
	defer f.Close()

	log.G(ctx).Info("creating EroFS archive")

	_, err = io.Copy(writer, f)
	if err != nil {
		return fmt.Errorf("could not copy EroFS data: %w", err)
	}

	return nil
}

func toArchivefsOptions(opts []ErofsCreateOption) ([]erofs.ErofsCreateOption, error) {
	var localOpts ErofsCreateOptions
	for _, opt := range opts {
		if err := opt(&localOpts); err != nil {
			return nil, err
		}
	}

	archivefsOpts := []erofs.ErofsCreateOption{
		erofs.WithAllRoot(localOpts.allRoot),
	}

	if localOpts.fInfoMap != nil {
		fInfoMap := make(map[string]erofs.FileInfo, len(localOpts.fInfoMap))
		for path, info := range localOpts.fInfoMap {
			fInfoMap[path] = erofs.FileInfo{
				Uid:  info.Uid,
				Gid:  info.Gid,
				Mode: info.Mode,
				Name: info.Name,
			}
		}
		archivefsOpts = append(archivefsOpts, erofs.WithFileInfoMap(fInfoMap))
	}

	return archivefsOpts, nil
}

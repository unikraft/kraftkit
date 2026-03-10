// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2025, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package cpio

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/anchore/stereoscope"
	scfile "github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
	"github.com/unikraft/go-cpio"

	"kraftkit.sh/fsutils"
	"kraftkit.sh/log"
)

type randSeq struct {
	number int32
}

// Int32 returns a random int32 value between 1000 and 4000. It is used to
// generate inode numbers for files in the CPIO archive.
func (r *randSeq) Int32() int32 {
	if r.number == 0 {
		r.number = rand.Int32()%3000 + 1000
	} else {
		r.number += 1
	}

	return r.number
}

type createOptions struct {
	opts CpioCreateOptions
}

func CreateFS(ctx context.Context, output string, source string, opts ...CpioCreateOption) error {
	c := &createOptions{}
	for _, opt := range opts {
		if err := opt(&c.opts); err != nil {
			return fmt.Errorf("could not apply CPIO create option: %w", err)
		}
	}

	f, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("could not open initramfs file: %w", err)
	}
	defer f.Close()

	writer := cpio.NewWriter(f)
	defer writer.Close()
	defer func() {
		if err := f.Sync(); err != nil {
			log.G(ctx).Errorf("syncing cpio archive failed: %s", err)
		}
	}()

	switch {
	case fsutils.IsOciArchive(source):
		if err := c.CreateFSFromOCIImage(ctx, writer, source); err != nil {
			return fmt.Errorf("could not create CPIO archive from OCI image: %w", err)
		}
	case fsutils.IsCpioFile(source):
		// If the source is already a CPIO file, we can simply copy its contents to the output.
		// This means we close the writer and re-open is as a simple byte writer.
		writer.Close()
		_, err := f.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("could not seek to start of CPIO archive: %w", err)
		}

		if err := c.CreateFSFromCpio(ctx, io.Writer(f), source); err != nil {
			return fmt.Errorf("could not create CPIO archive from CPIO file: %w", err)
		}
	case fsutils.IsErofsFile(source):
		return fmt.Errorf("creating CPIO from EroFS files is not currently supported")
	case fsutils.IsTarFile(source),
		fsutils.IsTarGzFile(source):
		if err := c.CreateFSFromTar(ctx, writer, source); err != nil {
			return fmt.Errorf("could not create CPIO archive from tar file: %w", err)
		}
	case fsutils.IsDirectory(source):
		if err := c.CreateFSFromDirectory(ctx, writer, source); err != nil {
			return fmt.Errorf("could not create CPIO archive from directory: %w", err)
		}
	default:
		return fmt.Errorf("unsupported source type: %s", source)
	}

	return nil
}

// CreateFSFromOCIImage creates a CPIO filesystem from an existing OCI image.
func (c *createOptions) CreateFSFromOCIImage(ctx context.Context, writer *cpio.Writer, source string) error {
	image, err := stereoscope.GetImage(ctx, source)
	if err != nil {
		return fmt.Errorf("could not load image: %w", err)
	}

	if err := image.SquashedTree().Walk(func(path scfile.Path, f filenode.FileNode) error {
		if f.Reference == nil {
			log.G(ctx).
				WithField("path", path).
				Debug("skipping: no reference")
			return nil
		}

		info, err := image.FileCatalog.Get(*f.Reference)
		if err != nil {
			return err
		}

		internal := fmt.Sprintf("./%s", path)
		if strings.HasPrefix(internal, ".//") {
			internal = internal[2:]
			internal = fmt.Sprintf(".%s", internal)
		}

		cpioHeader := &cpio.Header{
			Name:    internal,
			Mode:    cpio.FileMode(info.Mode().Perm()),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}

		// Populate platform specific information
		FileInfoToCPIOHeader(info, cpioHeader)

		switch f.FileType {
		case scfile.TypeBlockDevice:
			log.G(ctx).
				WithField("file", path).
				Warn("ignoring block devices")
			return nil

		case scfile.TypeCharacterDevice:
			log.G(ctx).
				WithField("file", path).
				Warn("ignoring char devices")
			return nil

		case scfile.TypeFIFO:
			log.G(ctx).
				WithField("file", path).
				Warn("ignoring fifo files")
			return nil

		case scfile.TypeSymLink:
			log.G(ctx).
				WithField("src", path).
				WithField("link", info.LinkDestination).
				Trace("symlinking")

			cpioHeader.Mode |= cpio.TypeSymlink
			cpioHeader.Linkname = info.LinkDestination
			cpioHeader.Size = int64(len(info.LinkDestination))

			if err := writer.WriteHeader(cpioHeader); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}

			if _, err := writer.Write([]byte(info.LinkDestination)); err != nil {
				return fmt.Errorf("could not write CPIO data for %s: %w", internal, err)
			}

		case scfile.TypeHardLink:
			log.G(ctx).
				WithField("src", path).
				WithField("link", info.LinkDestination).
				Trace("hardlinking")

			cpioHeader.Mode |= cpio.TypeRegular
			cpioHeader.Linkname = info.LinkDestination
			cpioHeader.Size = 0

			if err := writer.WriteHeader(cpioHeader); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}

		case scfile.TypeRegular:
			log.G(ctx).
				WithField("src", path).
				WithField("dst", internal).
				Trace("copying")

			cpioHeader.Mode |= cpio.TypeRegular
			cpioHeader.Linkname = info.LinkDestination
			cpioHeader.Size = info.Size()

			if err := writer.WriteHeader(cpioHeader); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}

			reader, err := image.OpenPathFromSquash(path)
			if err != nil {
				return fmt.Errorf("could not open file: %w", err)
			}

			data, err := io.ReadAll(reader)
			if err != nil {
				return fmt.Errorf("could not read file: %w", err)
			}

			if _, err := writer.Write(data); err != nil {
				return fmt.Errorf("could not write CPIO data for %s: %w", internal, err)
			}

		case scfile.TypeDirectory:
			log.G(ctx).
				WithField("dst", internal).
				Trace("mkdir")

			cpioHeader.Mode |= cpio.TypeDir

			return writer.WriteHeader(cpioHeader)

		default:
			log.G(ctx).
				WithField("file", path).
				WithField("type", f.FileType.String()).
				Warn("unsupported file type")
		}

		return nil
	}, &filetree.WalkConditions{
		LinkOptions: []filetree.LinkResolutionOption{},
		ShouldContinueBranch: func(path scfile.Path, f filenode.FileNode) bool {
			return f.LinkPath == ""
		},
	}); err != nil {
		return fmt.Errorf("could not walk image: %w", err)
	}

	return nil
}

// CreateFSFromTar creates a CPIO filesystem from an existing tar file.
func (c *createOptions) CreateFSFromTar(ctx context.Context, writer *cpio.Writer, source string) error {
	tarArchive, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("could not open output tarball: %w", err)
	}
	defer tarArchive.Close()

	// We create a Closer which is necessary to seek to zero in the tarball after
	// we've counted the number of links for the inode count.
	var close func() error

	var tarReader *tar.Reader
	if gzr, err := gzip.NewReader(tarArchive); err == nil {
		tarReader = tar.NewReader(gzr)
		close = gzr.Close
	} else {
		_, err = tarArchive.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("could not seek to start of tarball: %w", err)
		}

		tarReader = tar.NewReader(tarArchive)
		close = func() error { return nil }
	}

	type inodeCount struct {
		Count int
		Inode int32
	}
	fileCount := map[string]inodeCount{}
	randSeq := &randSeq{}

	// Pass once to count links
	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("could not read tar header on first pass: %w", err)
		}

		switch tarHeader.Typeflag {
		case tar.TypeLink:
			if _, ok := fileCount[tarHeader.Linkname]; !ok {
				fileCount[tarHeader.Linkname] = inodeCount{
					Count: 1,
					Inode: randSeq.Int32(),
				}
			} else {
				fileCount[tarHeader.Linkname] = inodeCount{
					Count: fileCount[tarHeader.Linkname].Count + 1,
					Inode: fileCount[tarHeader.Linkname].Inode,
				}
			}
		case tar.TypeReg:
			if _, ok := fileCount[tarHeader.Name]; !ok {
				fileCount[tarHeader.Name] = inodeCount{
					Count: 1,
					Inode: randSeq.Int32(),
				}
			} else {
				fileCount[tarHeader.Name] = inodeCount{
					Count: fileCount[tarHeader.Name].Count + 1,
					Inode: fileCount[tarHeader.Linkname].Inode,
				}
			}
		}
	}

	// Close the tarball and re-open it to read it again.
	if err := close(); err != nil {
		return err
	}

	_, err = tarArchive.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("could not seek to start of tarball: %w", err)
	}

	if gzr, err := gzip.NewReader(tarArchive); err == nil {
		tarReader = tar.NewReader(gzr)
	} else {
		_, err = tarArchive.Seek(0, io.SeekStart)
		if err != nil {
			return fmt.Errorf("could not seek to start of tarball: %w", err)
		}

		tarReader = tar.NewReader(tarArchive)
	}

	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			return fmt.Errorf("could not read tar header: %w", err)
		}

		internal := fmt.Sprintf("./%s", filepath.Clean(tarHeader.Name))

		if internal == "./." {
			continue
		}

		cpioHeader := &cpio.Header{
			Name:    internal,
			Mode:    cpio.FileMode(tarHeader.FileInfo().Mode().Perm()),
			ModTime: tarHeader.FileInfo().ModTime(),
			Size:    tarHeader.FileInfo().Size(),
		}

		switch tarHeader.Typeflag {
		case tar.TypeBlock:
			log.G(ctx).
				WithField("file", tarHeader.Name).
				Warn("ignoring block devices")
			continue

		case tar.TypeChar:
			log.G(ctx).
				WithField("file", tarHeader.Name).
				Warn("ignoring char devices")
			continue

		case tar.TypeFifo:
			log.G(ctx).
				WithField("file", tarHeader.Name).
				Warn("ignoring fifo files")
			continue

		case tar.TypeSymlink:
			log.G(ctx).
				WithField("src", tarHeader.Name).
				WithField("link", tarHeader.Linkname).
				Trace("symlinking")

			cpioHeader.Mode |= cpio.TypeSymlink
			cpioHeader.Linkname = tarHeader.Linkname
			cpioHeader.Size = int64(len(tarHeader.Linkname))

			if err := writer.WriteHeader(cpioHeader); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}

			if _, err := writer.Write([]byte(tarHeader.Linkname)); err != nil {
				return fmt.Errorf("could not write CPIO data for %s: %w", internal, err)
			}

		case tar.TypeLink:
			log.G(ctx).
				WithField("src", tarHeader.Name).
				WithField("link", tarHeader.Linkname).
				Trace("hardlinking")

			cpioHeader.Mode |= cpio.TypeRegular
			cpioHeader.Linkname = tarHeader.Linkname
			cpioHeader.Size = 0
			if _, ok := fileCount[tarHeader.Linkname]; ok {
				cpioHeader.Links = fileCount[tarHeader.Linkname].Count
				cpioHeader.Inode = int64(fileCount[tarHeader.Linkname].Inode)
			}
			if err := writer.WriteHeader(cpioHeader); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}

		case tar.TypeReg:
			log.G(ctx).
				WithField("src", tarHeader.Name).
				WithField("dst", internal).
				Trace("copying")

			cpioHeader.Mode |= cpio.TypeRegular
			cpioHeader.Linkname = tarHeader.Linkname
			cpioHeader.Size = tarHeader.FileInfo().Size()
			if _, ok := fileCount[tarHeader.Name]; ok {
				cpioHeader.Links = fileCount[tarHeader.Name].Count
				cpioHeader.Inode = int64(fileCount[tarHeader.Name].Inode)
			}

			if err := writer.WriteHeader(cpioHeader); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}

			data, err := io.ReadAll(tarReader)
			if err != nil {
				return fmt.Errorf("could not read file: %w", err)
			}

			if _, err := writer.Write(data); err != nil {
				return fmt.Errorf("could not write CPIO data for %s: %w", internal, err)
			}

		case tar.TypeDir:
			log.G(ctx).
				WithField("dst", internal).
				Trace("mkdir")

			cpioHeader.Mode |= cpio.TypeDir

			if err := writer.WriteHeader(cpioHeader); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}

		default:
			log.G(ctx).
				WithField("file", tarHeader.Name).
				WithField("type", tarHeader.Typeflag).
				Warn("unsupported file type")
		}
	}

	return nil
}

// CreateFSFromDirectory creates a CPIO filesystem from an existing directory.
func (c *createOptions) CreateFSFromDirectory(ctx context.Context, writer *cpio.Writer, source string) error {
	// Recursively walk and serialize to the output
	if err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("received error before parsing path: %w", err)
		}

		internal := strings.TrimPrefix(path, filepath.Clean(source))
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
			FileInfoToCPIOHeader(info, header)

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
		FileInfoToCPIOHeader(info, header)

		if c.opts.allRoot {
			header.Uid = 0
			header.Guid = 0
		}

		switch {
		case info.Mode().IsRegular():
			header.Mode |= cpio.TypeRegular

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
		return fmt.Errorf("could not walk output path: %w", err)
	}

	return nil
}

// CreateFSFromCpio creates a CPIO filesystem from an existing CPIO file.
func (c *createOptions) CreateFSFromCpio(ctx context.Context, writer io.Writer, source string) error {
	// Open and copy all contents from 'source' to the writer
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("could not open CPIO source file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(writer, f)
	if err != nil {
		return fmt.Errorf("could not copy CPIO data: %w", err)
	}

	return nil
}

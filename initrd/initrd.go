// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cavaliergopher/cpio"

	"kraftkit.sh/utils"
)

const InputDelimeter = ":"

type InitrdFormat int

const (
	ODC InitrdFormat = iota
	NEWC
	USTAR
)

func (d InitrdFormat) String() string {
	return [...]string{
		"odc",
		"newc",
		"ustar",
	}[d]
}

var NameToType = map[string]InitrdFormat{
	"odc":   ODC,
	"newc":  NEWC,
	"ustar": USTAR,
}

type InitrdConfig struct {
	Output   string            `yaml:"output,omitempty"`
	Files    map[string]string `yaml:"files,omitempty"`
	Format   InitrdFormat      `yaml:"format,omitempty"`
	Compress bool              `yaml:"compress,omitempty"`
	workdir  string
}

// NewFromFile interprets a given path as a initramfs disk file and returns an
// instantiated InitrdConfig if successful.
func NewFromFile(workdir, path string) (*InitrdConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	config := InitrdConfig{
		Output:  utils.RelativePath(workdir, path),
		Format:  NEWC,
		Files:   make(map[string]string),
		workdir: workdir,
	}
	reader := cpio.NewReader(file)

	// Iterate through the files in the archive.
	for {
		hdr, err := reader.Next()
		if err == io.EOF {
			// end of cpio archive
			break
		}
		if err != nil {
			return nil, err
		}

		config.Files[filepath.Join(workdir, hdr.Name)] = hdr.Name
	}

	return &config, nil
}

func (i *InitrdConfig) NewWriter(w io.Writer) (*cpio.Writer, error) {
	switch i.Format {
	case ODC:
		return nil, fmt.Errorf("unsupported CPIO format: odc")
	case NEWC:
		return cpio.NewWriter(w), nil
	case USTAR:
		return nil, fmt.Errorf("unsupported CPIO format: ustar")
	default:
		return nil, fmt.Errorf("unknown CPIO format")
	}
}

func (i *InitrdConfig) NewReader(w io.Reader) (*cpio.Reader, error) {
	switch i.Format {
	case ODC:
		return nil, fmt.Errorf("unsupported CPIO format: odc")
	case NEWC:
		return cpio.NewReader(w), nil
	case USTAR:
		return nil, fmt.Errorf("unsupported CPIO format: ustar")
	default:
		return nil, fmt.Errorf("unknown CPIO format")
	}
}

// NewFromPath accepts a positional argument base which is the directory that
// the provided files should be serialized from.  The prefix positional argument
// is used to affix all embedded files to the provided location.  Left empty,
// the defeault prefix is "/".
func NewFromPath(base string, prefix string, files ...string) (*InitrdConfig, error) {
	initrd := &InitrdConfig{
		Format: NEWC,
	}

	if prefix == "" {
		prefix = "/"
	}
	if !strings.HasPrefix(prefix, "/") {
		return nil, fmt.Errorf("must use absolute path in prefix: %s", prefix)
	}

	for _, file := range files {
		initrd.Files[utils.RelativePath(base, file)] = filepath.Join(prefix, file)
	}

	return initrd, nil
}

// NewFromMapping accepts a working directory base that represents the relative
// path of the files to serialize which are provided by a slice of mappings,
// each delimetered by the default delimeter `:` which separates the path on the
// host and the path inside of the iniramfs.  A destination of the final archive
// is provided by the positional argument output.
func NewFromMapping(workdir, output string, maps ...string) (*InitrdConfig, error) {
	initrd := InitrdConfig{
		workdir: workdir,
		Output:  output,
		Format:  NEWC,
		Files:   make(map[string]string),
	}

	f, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE, 0o664)
	if err != nil {
		return nil, fmt.Errorf("could not open initramfs file: %w", err)
	}

	writer := cpio.NewWriter(f)
	defer writer.Close()

	for _, mapping := range maps {
		split := strings.Split(mapping, InputDelimeter)
		if len(split) != 2 {
			return nil, fmt.Errorf("could not parse mapping '%s': must be in format <hostPath>:<initrdPath>", mapping)
		}

		hostPath := utils.RelativePath(workdir, split[0])
		initrdPath := split[1]

		f, err := os.Stat(hostPath)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("path does not exist: %s", hostPath)
		} else if err == nil && f.IsDir() {
			// The provided mapping is a directory, lets iterate over each path
			if err := filepath.WalkDir(hostPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				internal := strings.TrimPrefix(path, hostPath)
				if internal == "" {
					internal = "/"
				}
				if internal, err = filepath.Rel("/", internal); err != nil {
					return fmt.Errorf("trimming leading separator from path %q: %w", internal, err)
				}

				info, err := d.Info()
				if err != nil {
					return err
				}

				var link string
				if info.Mode()&os.ModeSymlink != 0 {
					if link, err = os.Readlink(path); err != nil {
						return err
					}
				}

				header, err := cpio.FileInfoHeader(info, link)
				if err != nil {
					return fmt.Errorf("generating cpio file info header: %w", err)
				}
				header.Name = internal

				if err := writer.WriteHeader(header); err != nil {
					return fmt.Errorf("writing cpio header for %q: %w", internal, err)
				}

				switch {
				case info.Mode()&fs.ModeSymlink != 0:
					initrd.Files[path] = internal
				case info.Mode().IsRegular():
					if err := copyData(func() (io.ReadCloser, error) { return os.Open(path) }, writer); err != nil {
						return fmt.Errorf("copying file %q to cpio archive: %w", path, err)
					}
					initrd.Files[path] = internal
				}

				return nil
			}); err != nil {
				return nil, fmt.Errorf("walking host path %s: %w", hostPath, err)
			}
		} else {
			header, err := cpio.FileInfoHeader(f, "")
			if err != nil {
				return nil, fmt.Errorf("generating cpio file info header: %w", err)
			}

			if err := writer.WriteHeader(header); err != nil {
				return nil, fmt.Errorf("writing cpio header for %q: %w", initrdPath, err)
			}

			if err := copyData(func() (io.ReadCloser, error) { return os.Open(hostPath) }, writer); err != nil {
				return nil, fmt.Errorf("copying file %q to cpio archive: %w", hostPath, err)
			}

			initrd.Files[initrdPath] = hostPath
		}
	}

	return &initrd, nil
}

// opener can return an arbitrary ReadCloser.
type opener func() (io.ReadCloser, error)

// copyData performs a copy from an io.Reader to the given io.Writer.
func copyData(open opener, w io.Writer) error {
	src, err := open()
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer src.Close()

	_, err = io.Copy(w, src)
	return err
}

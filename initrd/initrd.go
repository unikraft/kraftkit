// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.

package initrd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cavaliergopher/cpio"
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
	Output     string       `yaml:",omitempty" json:"output,omitempty"`
	Input      []string     `yaml:",omitempty" json:"input,omitempty"`
	Format     InitrdFormat `yaml:",omitempty" json:"format,omitempty"`
	Compress   bool         `yaml:",omitempty" json:"compress,omitempty"`
	WorkingDir string
	OutDir     string
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

// Files returns a map of files within the archive pointing to their absolute
// path on the host.
func (i *InitrdConfig) Files() (map[string]string, error) {
	files := make(map[string]string)

	for _, include := range i.Input {
		s := strings.Split(include, InputDelimeter)
		if len(s) > 2 {
			return nil, fmt.Errorf("invalid input format: expected \"host:archive\" and received: %s", include)
		}

		onHost, inArchive := i.RelativePath(s[0]), s[1]

		f, err := os.Stat(onHost)
		if err != nil {
			return nil, err
		}

		if f.IsDir() {
			if err := filepath.Walk(onHost, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				files[filepath.Join(inArchive, path)] = filepath.Join(onHost, path)

				return nil
			}); err != nil {
				return nil, err
			}
		} else {
			files[inArchive] = onHost
		}
	}

	return files, nil
}

// Build generates a new CPIO archive
func (i *InitrdConfig) Build(overwrite bool) error {
	if len(i.Output) == 0 {
		return fmt.Errorf("output path is required")
	}

	exists, err := os.Stat(i.Output)
	if err == nil && exists.IsDir() {
		return fmt.Errorf("cannot build CPIO archive as output cannot be directory")

		// If file already exists, is it a valid CPIO archive?
	} else if err == nil && !overwrite {
		file, err := os.Open(i.Output)
		if err != nil {
			return err
		}

		defer file.Close()

		reader, err := i.NewReader(file)
		if err != nil {
			return err
		}

		// Read the first header
		_, err = reader.Next()
		if err == cpio.ErrHeader {
			return fmt.Errorf("existing output archive is invalid: %s: %s", i.Output, err)
		}

		return nil
	}

	file, err := os.OpenFile(i.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}

	defer file.Close()

	// writer, err := i.NewWriter(file)

	return nil
}

// RelativePath resolve a relative path based the working directory
func (i *InitrdConfig) RelativePath(path string) string {
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}

	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(i.WorkingDir, path)
}

// ParseInitrdConfig parse short syntax for architecture configuration
func ParseInitrdConfig(workdir string, value string) (*InitrdConfig, error) {
	initrd := &InitrdConfig{
		Format: NEWC,
	}

	if len(value) == 0 {
		return initrd, fmt.Errorf("uninitialized value for initrd")
	}

	// Possible formats:
	//
	// ./path/on/disk (all files, we'll create a temp file)
	// ./path/on/disk:./filename.format (if format=cpio, select default)
	// ./path/on/diisk:./filename.formatz (-z suffix means compress)

	split := strings.Split(value, ":")
	if len(split) == 1 {
		initrd.Input = []string{
			fmt.Sprintf("%s:%s", workdir, split[0]),
		}
	} else if len(split) == 2 {
		initrd.Input = []string{
			fmt.Sprintf("%s:%s", workdir, split[0]),
		}
		initrd.Output = initrd.RelativePath(split[1])

		// TODO: if -z
	} else if len(split) > 2 {
		return initrd, fmt.Errorf("unexpected initrd format, expected path:file expression")
	}

	return initrd, nil
}

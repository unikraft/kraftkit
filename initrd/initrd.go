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

// NewFromFile interprets a given path as a initramfs disk file and returns an
// instantiated InitrdConfig if successful.
func NewFromFile(path string) (*InitrdConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	config := InitrdConfig{
		Output: path,
		Format: NEWC,
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

		config.Input = append(config.Input, hdr.Name)
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
	} else if len(split) > 2 {
		return initrd, fmt.Errorf("unexpected initrd format, expected path:file expression")
	}

	return initrd, nil
}

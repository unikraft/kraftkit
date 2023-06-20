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

	"kraftkit.sh/utils"

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
		Output:  path,
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

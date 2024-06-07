// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kraftkit.sh/log"

	"github.com/anchore/stereoscope"
	scfile "github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
	"github.com/cavaliergopher/cpio"
	"github.com/containers/image/v5/copy"
	ociarchive "github.com/containers/image/v5/oci/archive"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
)

type ociimage struct {
	imageName string
	opts      InitrdOptions
	args      []string
	ref       types.ImageReference
	env       []string
	files     []string
}

// NewFromOCIImage creates a new initrd from a remote container image.
func NewFromOCIImage(ctx context.Context, path string, opts ...InitrdOption) (Initrd, error) {
	if !strings.Contains("://", path) {
		path = fmt.Sprintf("docker://%s", path)
	}

	ref, err := alltransports.ParseImageName(path)
	if err != nil {
		return nil, err
	}

	initrd := ociimage{
		imageName: path,
		ref:       ref,
	}

	for _, opt := range opts {
		if err := opt(&initrd.opts); err != nil {
			return nil, err
		}
	}

	return &initrd, nil
}

// Build implements Initrd.
func (initrd *ociimage) Build(ctx context.Context) (string, error) {
	sysCtx := &types.SystemContext{
		OSChoice: "linux",
	}

	if initrd.opts.arch != "" {
		if initrd.opts.arch == "x86_64" {
			sysCtx.ArchitectureChoice = "amd64"
		} else {
			sysCtx.ArchitectureChoice = initrd.opts.arch
		}
	}

	policy := &signature.Policy{
		Default: []signature.PolicyRequirement{
			signature.NewPRInsecureAcceptAnything(),
		},
	}

	policyCtx, err := signature.NewPolicyContext(policy)
	if err != nil {
		return "", fmt.Errorf("failed to generate default policy context: %w", err)
	}

	defer func() {
		_ = policyCtx.Destroy()
	}()

	img, err := initrd.ref.NewImage(ctx, sysCtx)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = img.Close()
	}()

	ociImage, err := img.OCIConfig(ctx)
	if err != nil {
		return "", err
	}

	initrd.args = ociImage.Config.Entrypoint
	initrd.args = append(initrd.args, ociImage.Config.Cmd...)
	initrd.env = ociImage.Config.Env

	if initrd.opts.output == "" {
		fi, err := os.CreateTemp("", "")
		if err != nil {
			return "", err
		}

		initrd.opts.output = fi.Name()
	}

	// Create a temporary directory to output the image to
	outputDir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", fmt.Errorf("could not make temporary directory: %w", err)
	}

	defer func() {
		_ = os.RemoveAll(outputDir)
	}()

	ociTarballFile := filepath.Join(outputDir, "oci.tar.gz")

	dest, err := ociarchive.NewReference(ociTarballFile, "")
	if err != nil {
		return "", fmt.Errorf("invalid destination name %s: %v", dest, err)
	}

	opts := copy.Options{
		ReportWriter:   log.G(ctx).Writer(),
		DestinationCtx: sysCtx,
		SourceCtx:      sysCtx,
	}

	if _, err = copy.Image(ctx, policyCtx, dest, initrd.ref, &opts); err != nil {
		return "", fmt.Errorf("failed to copy image: %w", err)
	}

	image, err := stereoscope.GetImage(ctx, ociTarballFile)
	if err != nil {
		return "", fmt.Errorf("could not load image: %w", err)
	}

	f, err := os.OpenFile(initrd.opts.output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("could not open initramfs file: %w", err)
	}

	defer func() {
		_ = f.Close()
	}()

	writer := cpio.NewWriter(f)

	defer func() {
		_ = writer.Close()
	}()

	if err := image.SquashedTree().Walk(func(path scfile.Path, f filenode.FileNode) error {
		if f.Reference == nil {
			log.G(ctx).
				WithField("path", path).
				Warn("skipping: no reference")
			return nil
		}

		info, err := image.FileCatalog.Get(*f.Reference)
		if err != nil {
			return err
		}

		internal := fmt.Sprintf(".%s", path)

		if f.FileType == scfile.TypeDirectory {
			if err := writer.WriteHeader(&cpio.Header{
				Name: internal,
				Mode: cpio.FileMode(info.Mode().Perm()) | cpio.TypeDir,
			}); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}

			return nil
		}

		initrd.files = append(initrd.files, internal)

		log.G(ctx).
			WithField("file", path).
			Trace("archiving")

		var data []byte

		header := &cpio.Header{
			Name:    internal,
			Mode:    cpio.FileMode(info.Mode().Perm()),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}

		// Populate platform specific information
		populateCPIO(info, header)

		// No other file types are part of the archive.  As a result we only check
		// whether the path is the same as the reference path which indicates
		// whether the entry is a symbolic link.
		if f.FileType == scfile.TypeSymLink {
			header.Mode |= cpio.TypeSymlink
			header.Linkname = info.Path
			header.Size = int64(len(info.Path))
			data = []byte(info.Path)
		} else {
			reader, err := image.OpenPathFromSquash(path)
			if err != nil {
				return fmt.Errorf("could not open file: %w", err)
			}

			header.Mode |= cpio.TypeReg
			header.Size = info.Size()
			data, err = io.ReadAll(reader)
			if err != nil {
				return fmt.Errorf("could not read file: %w", err)
			}
		}

		if err := writer.WriteHeader(header); err != nil {
			return fmt.Errorf("writing cpio header for %q: %w", internal, err)
		}

		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("could not write CPIO data for %s: %w", internal, err)
		}

		return nil
	}, nil); err != nil {
		return "", fmt.Errorf("could not walk image: %w", err)
	}

	if initrd.opts.compress {
		if err := compressFiles(initrd.opts.output, writer, f); err != nil {
			return "", fmt.Errorf("could not compress files: %w", err)
		}
	}

	return initrd.opts.output, nil
}

// Files implements Initrd.
func (initrd *ociimage) Files() []string {
	return initrd.files
}

// Env implements Initrd.
func (initrd *ociimage) Env() []string {
	return initrd.env
}

// Args implements Initrd.
func (initrd *ociimage) Args() []string {
	return initrd.args
}

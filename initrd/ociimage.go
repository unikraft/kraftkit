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

	"kraftkit.sh/cpio"
	"kraftkit.sh/log"

	"github.com/anchore/stereoscope"
	scfile "github.com/anchore/stereoscope/pkg/file"
	"github.com/anchore/stereoscope/pkg/filetree"
	"github.com/anchore/stereoscope/pkg/filetree/filenode"
	"github.com/containers/image/v5/copy"
	ociarchive "github.com/containers/image/v5/oci/archive"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type ociimage struct {
	imageName string
	opts      InitrdOptions
	args      []string
	ref       types.ImageReference
	env       []string
}

// NewFromOCIImage creates a new initrd from a remote container image.
func NewFromOCIImage(ctx context.Context, path string, opts ...InitrdOption) (Initrd, error) {
	var transport string
	if strings.Contains(path, "://") {
		transport, path, _ = strings.Cut(path, "://")
	}

	nref, err := name.ParseReference(path)
	if err != nil {
		return nil, err
	}

	if desc, err := remote.Head(nref); err != nil || desc == nil {
		return nil, fmt.Errorf("could not find image: %w", err)
	}

	if !strings.Contains("://", path) {
		path = fmt.Sprintf("docker://%s", path)
	} else {
		path = fmt.Sprintf("%s://%s", transport, path)
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

	if initrd.opts.arch == "x86_64" {
		sysCtx.ArchitectureChoice = "amd64"
	} else if initrd.opts.arch != "" {
		sysCtx.ArchitectureChoice = initrd.opts.arch
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

	initrd.args = append(ociImage.Config.Entrypoint,
		ociImage.Config.Cmd...,
	)
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

	log.G(ctx).
		WithField("image", initrd.ref.StringWithinTransport()).
		Debug("pulling")

	if _, err = copy.Image(ctx, policyCtx, dest, initrd.ref, &opts); err != nil {
		return "", fmt.Errorf("failed to copy image: %w", err)
	}

	image, err := stereoscope.GetImage(ctx, ociTarballFile)
	if err != nil {
		return "", fmt.Errorf("could not load image: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(initrd.opts.output), 0o755); err != nil {
		return "", fmt.Errorf("could not create output directory: %w", err)
	}

	f, err := os.OpenFile(initrd.opts.output, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("could not open initramfs file: %w", err)
	}

	defer func() {
		_ = f.Close()
	}()

	cpioWriter := cpio.NewWriter(f)

	defer func() {
		_ = cpioWriter.Close()
	}()

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

		internal := filepath.Clean(fmt.Sprintf("/%s", path))

		cpioHeader := &cpio.Header{
			Name:    internal,
			Mode:    cpio.FileMode(info.Mode().Perm()),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}

		// Populate platform specific information
		populateCPIO(info, cpioHeader)

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
				Debug("symlinking")

			cpioHeader.Mode |= cpio.TypeSymlink
			cpioHeader.Linkname = info.LinkDestination
			cpioHeader.Size = int64(len(info.LinkDestination))

			if err := cpioWriter.WriteHeader(cpioHeader); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}

			if _, err := cpioWriter.Write([]byte(info.LinkDestination)); err != nil {
				return fmt.Errorf("could not write CPIO data for %s: %w", internal, err)
			}

		case scfile.TypeHardLink:
			log.G(ctx).
				WithField("src", path).
				WithField("link", info.LinkDestination).
				Debug("hardlinking")

			cpioHeader.Mode |= cpio.TypeReg
			cpioHeader.Linkname = info.LinkDestination
			cpioHeader.Size = 0

			if err := cpioWriter.WriteHeader(cpioHeader); err != nil {
				return fmt.Errorf("could not write CPIO header: %w", err)
			}

		case scfile.TypeRegular:
			log.G(ctx).
				WithField("src", path).
				WithField("dst", internal).
				Debug("copying")

			cpioHeader.Mode |= cpio.TypeReg
			cpioHeader.Linkname = info.LinkDestination
			cpioHeader.Size = info.Size()

			if err := cpioWriter.WriteHeader(cpioHeader); err != nil {
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

			if _, err := cpioWriter.Write(data); err != nil {
				return fmt.Errorf("could not write CPIO data for %s: %w", internal, err)
			}

		case scfile.TypeDirectory:
			log.G(ctx).
				WithField("dst", internal).
				Debug("mkdir")

			cpioHeader.Mode |= cpio.TypeDir

			return cpioWriter.WriteHeader(cpioHeader)

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
		return "", fmt.Errorf("could not walk image: %w", err)
	}

	if initrd.opts.compress {
		if err := compressFiles(initrd.opts.output, cpioWriter, f); err != nil {
			return "", fmt.Errorf("could not compress files: %w", err)
		}
	}

	return initrd.opts.output, nil
}

// Env implements Initrd.
func (initrd *ociimage) Env() []string {
	return initrd.env
}

// Args implements Initrd.
func (initrd *ociimage) Args() []string {
	return initrd.args
}

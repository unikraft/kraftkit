//go:build !openbsd && !netbsd
// +build !openbsd,!netbsd

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kraftkit.sh/fs/cpio"
	"kraftkit.sh/fs/erofs"
	"kraftkit.sh/log"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"go.podman.io/image/v5/copy"
	ociarchive "go.podman.io/image/v5/oci/archive"
	"go.podman.io/image/v5/signature"
	"go.podman.io/image/v5/transports/alltransports"
	"go.podman.io/image/v5/types"
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
		opts: InitrdOptions{
			fsType: FsTypeCpio,
		},
	}

	for _, opt := range opts {
		if err := opt(&initrd.opts); err != nil {
			return nil, err
		}
	}

	return &initrd, nil
}

// Build implements Initrd.
func (initrd *ociimage) Name() string {
	return "OCI image"
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

	if err := os.MkdirAll(filepath.Dir(initrd.opts.output), 0o755); err != nil {
		return "", fmt.Errorf("could not create output directory: %w", err)
	}

	switch initrd.opts.fsType {
	case FsTypeErofs:
		return initrd.opts.output, erofs.CreateFS(ctx, initrd.opts.output, ociTarballFile,
			erofs.WithAllRoot(!initrd.opts.keepOwners),
		)
	case FsTypeCpio:
		err := cpio.CreateFS(ctx, initrd.opts.output, ociTarballFile,
			cpio.WithAllRoot(!initrd.opts.keepOwners),
		)
		if err != nil {
			return "", fmt.Errorf("could not create CPIO archive: %w", err)
		}
		if initrd.opts.compress {
			if err := compressFiles(initrd.opts.output, initrd.opts.output); err != nil {
				return "", fmt.Errorf("could not compress files: %w", err)
			}
		}

		return initrd.opts.output, nil
	default:
		return "", fmt.Errorf("unknown filesystem type %s", initrd.opts.fsType)
	}
}

// Options implements Initrd.
func (initrd *ociimage) Options() InitrdOptions {
	return initrd.opts
}

// Env implements Initrd.
func (initrd *ociimage) Env() []string {
	return initrd.env
}

// Args implements Initrd.
func (initrd *ociimage) Args() []string {
	return initrd.args
}

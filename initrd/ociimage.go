//go:build !openbsd && !netbsd
// +build !openbsd,!netbsd

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"kraftkit.sh/fs/cpio"
	"kraftkit.sh/fs/erofs"
	"kraftkit.sh/log"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/images/archive"
	"github.com/containerd/platforms"
	"github.com/moby/buildkit/util/contentutil"
	"github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

type ociimage struct {
	imageName string
	opts      InitrdOptions
	args      []string
	desc      ocispecs.Descriptor
	provider  content.InfoReaderProvider
	env       []string
}

// NewFromOCIImage creates a new initrd from a remote container image.
func NewFromOCIImage(ctx context.Context, path string, opts ...InitrdOption) (Initrd, error) {
	desc, provider, err := contentutil.ProviderFromRef(path)
	if err != nil {
		return nil, fmt.Errorf("could not find image: %w", err)
	}

	initrd := ociimage{
		imageName: path,
		desc:      desc,
		provider:  noopInfoReaderProvider{provider},
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
	var sys ocispecs.Platform
	if initrd.opts.arch == "x86_64" {
		sys = ocispecs.Platform{
			OS:           "linux",
			Architecture: "amd64",
		}
	} else if initrd.opts.arch != "" {
		sys = ocispecs.Platform{
			OS:           "linux",
			Architecture: initrd.opts.arch,
		}
	}

	configDesc, err := images.Config(ctx, initrd.provider, initrd.desc, platforms.Only(sys))
	if err != nil {
		return "", fmt.Errorf("could not get image config: %w", err)
	}
	config, err := content.ReadBlob(ctx, initrd.provider, configDesc)
	if err != nil {
		return "", fmt.Errorf("could not read image config: %w", err)
	}
	var cfg ocispecs.Image
	err = json.Unmarshal(config, &cfg)
	if err != nil {
		return "", fmt.Errorf("could not unmarshal image config: %w", err)
	}

	initrd.args = slices.Concat(cfg.Config.Entrypoint, cfg.Config.Cmd)
	initrd.env = cfg.Config.Env

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

	ociTarballFile, err := os.Create(filepath.Join(outputDir, "oci.tar.gz"))
	if err != nil {
		return "", fmt.Errorf("could not create OCI tarball file: %w", err)
	}

	log.G(ctx).
		WithField("image", initrd.imageName).
		Debug("pulling")

	err = archive.Export(ctx, initrd.provider, ociTarballFile, archive.WithManifest(initrd.desc), archive.WithPlatform(platforms.Only(sys)))
	if err != nil {
		return "", fmt.Errorf("could not export image to OCI tarball: %w", err)
	}
	if err := ociTarballFile.Close(); err != nil {
		return "", fmt.Errorf("could not close OCI tarball file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(initrd.opts.output), 0o755); err != nil {
		return "", fmt.Errorf("could not create output directory: %w", err)
	}

	switch initrd.opts.fsType {
	case FsTypeErofs:
		return initrd.opts.output, erofs.CreateFS(ctx, initrd.opts.output, ociTarballFile.Name(),
			erofs.WithAllRoot(!initrd.opts.keepOwners),
		)
	case FsTypeCpio:
		err := cpio.CreateFS(ctx, initrd.opts.output, ociTarballFile.Name(),
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

type noopInfoReaderProvider struct {
	content.Provider
}

func (n noopInfoReaderProvider) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	return content.Info{
		Digest: dgst,
	}, nil
}

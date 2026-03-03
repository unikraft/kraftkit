//go:build !openbsd && !netbsd
// +build !openbsd,!netbsd

// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package initrd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	v1tarball "github.com/google/go-containerregistry/pkg/v1/tarball"

	"kraftkit.sh/config"
	"kraftkit.sh/fs/cpio"
	"kraftkit.sh/fs/erofs"
	"kraftkit.sh/internal/version"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/cache"
	"kraftkit.sh/oci/simpleauth"
)

type ociimage struct {
	imageName string
	opts      InitrdOptions
	args      []string
	env       []string
	auths     map[string]config.AuthConfig
}

// NewFromOCIImage creates a new initrd from a remote container image.
func NewFromOCIImage(ctx context.Context, path string, opts ...InitrdOption) (Initrd, error) {
	// Strip protocol prefix if present (OCI references don't use http:// or https://)
	path = stripProtocolPrefix(path)

	// Parse the reference to validate it
	_, err := name.ParseReference(path)
	if err != nil {
		return nil, fmt.Errorf("could not parse image reference: %w", err)
	}

	initrd := ociimage{
		imageName: path,
		opts: InitrdOptions{
			fsType: FsTypeCpio,
		},
	}

	for _, opt := range opts {
		if err := opt(&initrd.opts); err != nil {
			return nil, err
		}
	}

	// Get authentication config
	if initrd.opts.auths == nil {
		initrd.auths = config.G[config.KraftKit](ctx).Auth
	} else {
		initrd.auths = initrd.opts.auths
	}

	return &initrd, nil
}

// Build implements Initrd.
func (initrd *ociimage) Name() string {
	return "OCI image"
}

// Build implements Initrd.
func (initrd *ociimage) Build(ctx context.Context) (string, error) {
	// Parse the image reference
	ref, err := name.ParseReference(initrd.imageName)
	if err != nil {
		return "", fmt.Errorf("could not parse image reference: %w", err)
	}

	// Setup remote options
	authConfig := &authn.AuthConfig{}
	ropts := []remote.Option{
		remote.WithContext(ctx),
		remote.WithUserAgent(version.UserAgent()),
	}

	// Configure platform
	arch := initrd.opts.arch
	if arch == "x86_64" {
		arch = "amd64"
	}
	if arch != "" {
		ropts = append(ropts, remote.WithPlatform(v1.Platform{
			OS:           "linux",
			Architecture: arch,
		}))
	}

	// Configure authentication
	if auth, ok := initrd.auths[ref.Context().RegistryStr()]; ok {
		authConfig.Username = auth.User
		authConfig.Password = auth.Token

		ropts = append(ropts,
			remote.WithAuth(&simpleauth.SimpleAuthenticator{
				Auth: authConfig,
			}),
		)

		if !auth.VerifySSL {
			var transport *http.Transport
			if t, ok := remote.DefaultTransport.(*http.Transport); ok {
				transport = t.Clone()
			} else if t, ok := http.DefaultTransport.(*http.Transport); ok {
				transport = t.Clone()
			} else {
				transport = &http.Transport{}
			}
			transport.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
			ropts = append(ropts, remote.WithTransport(transport))

			// Re-parse the reference with the Insecure option to allow fetching
			// from registries with invalid TLS certificates.
			ref, err = name.ParseReference(initrd.imageName, name.Insecure)
			if err != nil {
				return "", fmt.Errorf("could not parse image reference: %w", err)
			}
		}
	}

	log.G(ctx).
		WithField("image", initrd.imageName).
		Debug("pulling")

	// Pull the image
	img, err := cache.RemoteImage(ref, ropts...)
	if err != nil {
		return "", fmt.Errorf("could not pull image: %w", err)
	}

	// Get the image config
	configFile, err := img.ConfigFile()
	if err != nil {
		return "", fmt.Errorf("could not get image config: %w", err)
	}

	initrd.args = slices.Concat(configFile.Config.Entrypoint, configFile.Config.Cmd)
	initrd.env = configFile.Config.Env

	if initrd.opts.output == "" {
		fi, err := os.CreateTemp("", "")
		if err != nil {
			return "", err
		}
		initrd.opts.output = fi.Name()

		if err := fi.Close(); err != nil {
			return "", err
		}
	}

	// Create a temporary directory to output the image to
	outputDir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", fmt.Errorf("could not make temporary directory: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(outputDir)
	}()

	ociTarballPath := filepath.Join(outputDir, "oci.tar")

	log.G(ctx).
		WithField("image", initrd.imageName).
		Debug("exporting to tarball")

	// Write the image to a tarball
	if err := v1tarball.WriteToFile(ociTarballPath, ref, img); err != nil {
		return "", fmt.Errorf("could not write image to tarball: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(initrd.opts.output), 0o755); err != nil {
		return "", fmt.Errorf("could not create output directory: %w", err)
	}

	switch initrd.opts.fsType {
	case FsTypeFile:
		return "", fmt.Errorf("cannot build initrd from oci image with file output type")
	case FsTypeErofs:
		return initrd.opts.output, erofs.CreateFS(ctx, initrd.opts.output, ociTarballPath,
			erofs.WithAllRoot(!initrd.opts.keepOwners),
		)
	case FsTypeCpio:
		err := cpio.CreateFS(ctx, initrd.opts.output, ociTarballPath,
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

// stripProtocolPrefix removes http:// https:// oci:// prefixes from an image reference.
// OCI registry references should not include protocol prefixes.
func stripProtocolPrefix(ref string) string {
	if after, ok := strings.CutPrefix(ref, "https://"); ok {
		return after
	}
	if after, ok := strings.CutPrefix(ref, "http://"); ok {
		return after
	}
	if after, ok := strings.CutPrefix(ref, "oci://"); ok {
		return after
	}
	return ref
}

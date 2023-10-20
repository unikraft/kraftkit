// SPDX-License-Identifier: BSD-3-Clause
// Copyright (c) 2022, Unikraft GmbH and The KraftKit Authors.
// Licensed under the BSD-3-Clause License (the "License").
// You may not use this file except in compliance with the License.
package oci

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"

	"kraftkit.sh/config"
	"kraftkit.sh/initrd"
	"kraftkit.sh/internal/set"
	"kraftkit.sh/internal/tableprinter"
	kraftkitversion "kraftkit.sh/internal/version"
	"kraftkit.sh/kconfig"
	"kraftkit.sh/log"
	"kraftkit.sh/oci/handler"
	"kraftkit.sh/oci/simpleauth"
	"kraftkit.sh/pack"
	"kraftkit.sh/packmanager"
	"kraftkit.sh/unikraft"
	"kraftkit.sh/unikraft/arch"
	"kraftkit.sh/unikraft/plat"
	"kraftkit.sh/unikraft/target"
)

const ConfigFilename = "config.json"

// ociPackage works by referencing a specific manifest which represents the
// "package" as well as the index that manifest should be part of.  When
// when internally referencing the packaged entity, this is the manifest and its
// representation is presented via the index.
type ociPackage struct {
	handle   handler.Handler
	ref      name.Reference
	index    *Index
	manifest *Manifest

	// Embedded attributes which represent target.Target
	arch    arch.Architecture
	plat    plat.Platform
	kconfig kconfig.KeyValueMap
	kernel  string
	initrd  initrd.Initrd
	command []string
}

var (
	_ pack.Package  = (*ociPackage)(nil)
	_ target.Target = (*ociPackage)(nil)
)

// NewPackageFromTarget generates an OCI implementation of the pack.Package
// construct based on an input Application and options.
func NewPackageFromTarget(ctx context.Context, targ target.Target, opts ...packmanager.PackOption) (pack.Package, error) {
	var err error

	popts := packmanager.NewPackOptions()
	for _, opt := range opts {
		opt(popts)
	}

	// Initialize the ociPackage by copying over target.Target attributes
	ocipack := ociPackage{
		arch:    targ.Architecture(),
		plat:    targ.Platform(),
		kconfig: targ.KConfig(),
		kernel:  targ.Kernel(),
		initrd:  targ.Initrd(),
		command: popts.Args(),
	}

	if popts.Name() == "" {
		return nil, fmt.Errorf("cannot create package without name")
	}
	ocipack.ref, err = name.ParseReference(
		popts.Name(),
		name.WithDefaultRegistry(""),
	)
	if err != nil {
		return nil, fmt.Errorf("could not parse image reference: %w", err)
	}

	auths, err := defaultAuths(ctx)
	if err != nil {
		return nil, err
	}

	if contAddr := config.G[config.KraftKit](ctx).ContainerdAddr; len(contAddr) > 0 {
		namespace := DefaultNamespace
		if n := os.Getenv("CONTAINERD_NAMESPACE"); n != "" {
			namespace = n
		}

		log.G(ctx).WithFields(logrus.Fields{
			"addr":      contAddr,
			"namespace": namespace,
		}).Debug("oci: packaging via containerd")

		ctx, ocipack.handle, err = handler.NewContainerdHandler(ctx, contAddr, namespace, auths)
	} else {
		if gerr := os.MkdirAll(config.G[config.KraftKit](ctx).RuntimeDir, fs.ModeSetgid|0o775); gerr != nil {
			return nil, fmt.Errorf("could not create local oci cache directory: %w", gerr)
		}

		ociDir := filepath.Join(config.G[config.KraftKit](ctx).RuntimeDir, "oci")

		log.G(ctx).WithFields(logrus.Fields{
			"path": ociDir,
		}).Trace("oci: directory handler")

		ocipack.handle, err = handler.NewDirectoryHandler(ociDir, auths)
	}
	if err != nil {
		return nil, err
	}

	// Prepare a new manifest which contains the individual components of the
	// target, including the kernel image.
	ocipack.manifest, err = NewManifest(ctx, ocipack.handle)
	if err != nil {
		return nil, fmt.Errorf("could not instantiate new manifest structure: %w", err)
	}

	log.G(ctx).WithFields(logrus.Fields{
		"src":  ocipack.Kernel(),
		"dest": WellKnownKernelPath,
	}).Debug("oci: including kernel")

	layer, err := NewLayerFromFile(ctx,
		ocispec.MediaTypeImageLayer,
		ocipack.Kernel(),
		WellKnownKernelPath,
		WithLayerAnnotation(AnnotationKernelPath, WellKnownKernelPath),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create new layer structure from file: %w", err)
	}
	defer os.Remove(layer.tmp)

	if _, err := ocipack.manifest.AddLayer(ctx, layer); err != nil {
		return nil, fmt.Errorf("could not add layer to manifest: %w", err)
	}

	if popts.Initrd() != "" {
		log.G(ctx).
			WithField("src", popts.Initrd()).
			WithField("dest", WellKnownInitrdPath).
			Debug("oci: including initrd")

		layer, err := NewLayerFromFile(ctx,
			ocispec.MediaTypeImageLayer,
			popts.Initrd(),
			WellKnownInitrdPath,
			WithLayerAnnotation(AnnotationKernelInitrdPath, WellKnownInitrdPath),
		)
		if err != nil {
			return nil, fmt.Errorf("could build layer from file: %w", err)
		}
		defer os.Remove(layer.tmp)

		if _, err := ocipack.manifest.AddLayer(ctx, layer); err != nil {
			return nil, err
		}
	}

	// TODO(nderjung): See below.

	// if popts.PackKernelLibraryObjects() {
	// 	log.G(ctx).Debug("oci: including kernel library objects")
	// }

	// if popts.PackKernelLibraryIntermediateObjects() {
	// 	log.G(ctx).Debug("oci: including kernel library intermediate objects")
	// }

	// if popts.PackKernelSourceFiles() {
	// 	log.G(ctx).Debug("oci: including kernel source files")
	// }

	// if popts.PackAppSourceFiles() {
	// 	log.G(ctx).Debug("oci: including application source files")
	// }

	ocipack.manifest.SetAnnotation(ctx, AnnotationName, ocipack.Name())
	ocipack.manifest.SetAnnotation(ctx, AnnotationKraftKitVersion, kraftkitversion.Version())
	if version := popts.KernelVersion(); len(version) > 0 {
		ocipack.manifest.SetAnnotation(ctx, AnnotationKernelVersion, version)
		ocipack.manifest.SetOSVersion(ctx, version)
	}

	ocipack.manifest.SetCmd(ctx, ocipack.Command())
	ocipack.manifest.SetOS(ctx, ocipack.Platform().Name())
	ocipack.manifest.SetArchitecture(ctx, ocipack.Architecture().Name())

	var index *Index

	switch popts.MergeStrategy() {
	case packmanager.StrategyMerge, packmanager.StrategyExit:
		index, err = NewIndexFromRef(ctx, ocipack.handle, ocipack.ref.String())
		if err != nil {
			index, err = NewIndex(ctx, ocipack.handle)
			if err != nil {
				return nil, fmt.Errorf("could not instantiate new image structure: %w", err)
			}
		} else if popts.MergeStrategy() == packmanager.StrategyExit {
			return nil, fmt.Errorf("cannot overwrite existing manifest as merge strategy is set to exit on conflict")
		}

	case packmanager.StrategyOverwrite:
		if err := ocipack.handle.DeleteIndex(ctx, ocipack.ref.String()); err != nil {
			return nil, fmt.Errorf("could not remove existing index: %w", err)
		}

		index, err = NewIndex(ctx, ocipack.handle)
		if err != nil {
			return nil, fmt.Errorf("could not instantiate new image structure: %w", err)
		}
	}

	if popts.MergeStrategy() == packmanager.StrategyExit && len(index.manifests) > 0 {
		return nil, fmt.Errorf("cannot continue: reference already exists and merge strategy set to none")
	}

	if len(index.manifests) > 0 {
		// Sort the features alphabetically.  This ensures that comparisons between
		// versions are symmetric.
		sort.Slice(ocipack.manifest.config.OSFeatures, func(i, j int) bool {
			// Check if we have numbers, sort them accordingly
			if z, err := strconv.Atoi(ocipack.manifest.config.OSFeatures[i]); err == nil {
				if y, err := strconv.Atoi(ocipack.manifest.config.OSFeatures[j]); err == nil {
					return y < z
				}
				// If we get only one number, alway say its greater than letter
				return true
			}
			// Compare letters normally
			return ocipack.manifest.config.OSFeatures[j] > ocipack.manifest.config.OSFeatures[i]
		})

		newManifestChecksum, err := PlatformChecksum(ocipack.ref.String(), &ocispec.Platform{
			Architecture: ocipack.manifest.config.Architecture,
			OS:           ocipack.manifest.config.OS,
			OSVersion:    ocipack.manifest.config.OSVersion,
			OSFeatures:   ocipack.manifest.config.OSFeatures,
		})
		if err != nil {
			return nil, fmt.Errorf("could not generate manifest platform checksum: %w", err)
		}

		var manifests []*Manifest

		for _, existingManifest := range index.manifests {
			existingManifestChecksum, err := PlatformChecksum(ocipack.ref.String(), &ocispec.Platform{
				Architecture: existingManifest.config.Architecture,
				OS:           existingManifest.config.OS,
				OSVersion:    existingManifest.config.OSVersion,
				OSFeatures:   existingManifest.config.OSFeatures,
			})
			if err != nil {
				return nil, fmt.Errorf("could not generate manifest platform checksum for '%s': %w", existingManifest.desc.Digest.String(), err)
			}
			if existingManifestChecksum == newManifestChecksum {
				switch popts.MergeStrategy() {
				case packmanager.StrategyExit:
					return nil, fmt.Errorf("cannot overwrite existing manifest as merge strategy is set to exit on conflict")

				// A manifest with the same configuration has been detected, in
				// both cases,
				case packmanager.StrategyOverwrite, packmanager.StrategyMerge:
					if err := ocipack.handle.DeleteManifest(ctx, ocipack.ref.Name(), existingManifest.desc.Digest); err != nil {
						return nil, fmt.Errorf("could not overwrite existing manifest: %w", err)
					}
				}
			} else {
				manifests = append(manifests, existingManifest)
			}
		}

		index.manifests = manifests
	}

	index.SetAnnotation(ctx, AnnotationKraftKitVersion, kraftkitversion.Version())

	if popts.PackKConfig() {
		log.G(ctx).Debug("oci: including list of kconfig as features")

		// TODO(nderjung): Not sure if these filters are best placed here or
		// elsewhere.
		skippable := set.NewStringSet(
			"CONFIG_UK_APP",
			"CONFIG_UK_BASE",
		)
		for _, k := range ocipack.KConfig() {
			// Filter out host-specific KConfig options.
			if skippable.Contains(k.Key) {
				continue
			}

			log.G(ctx).
				WithField(k.Key, k.Value).
				Trace("oci: feature")

			ocipack.manifest.SetOSFeature(ctx, k.String())
		}
	}

	if err := index.AddManifest(ctx, ocipack.manifest); err != nil {
		return nil, fmt.Errorf("could not add manifest to index: %w", err)
	}

	if _, err = index.Save(ctx, ocipack.ref.String(), nil); err != nil {
		return nil, fmt.Errorf("could not save index: %w", err)
	}

	return &ocipack, nil
}

// NewPackageFromOCIManifestDigest is a constructor method which
// instantiates a package based on the OCI format based on a provided OCI
// Image manifest digest.
func NewPackageFromOCIManifestDigest(ctx context.Context, handle handler.Handler, ref string, auths map[string]config.AuthConfig, dgst digest.Digest) (pack.Package, error) {
	var err error

	ocipack := ociPackage{
		handle: handle,
	}

	ocipack.ref, err = name.ParseReference(ref,
		name.WithDefaultRegistry(""),
	)
	if err != nil {
		return nil, err
	}

	// First, check if the digest exists locally, this determines whether we
	// continue to instantiate it from the local host or from from a remote
	// registry.
	if exists, err := handle.DigestExists(ctx, dgst); err == nil && exists {
		ocipack.index, err = NewIndexFromRef(ctx, handle, ref)
		if err != nil {
			return nil, fmt.Errorf("could not instantiate index from reference: %w", err)
		}

		manifest, err := NewManifestFromDigest(ctx, handle, dgst)
		if err != nil {
			return nil, fmt.Errorf("could not instantiate manifest from digest: %w", err)
		}

		if err := ocipack.index.AddManifest(ctx, manifest); err != nil {
			return nil, fmt.Errorf("could not add manifest to index: %w", err)
		}
		ocipack.manifest = manifest
	} else {
		if ocipack.ref.Context().RegistryStr() == "" {
			ocipack.ref, err = name.ParseReference(ref,
				name.WithDefaultRegistry(DefaultRegistry),
			)
			if err != nil {
				return nil, err
			}
		}

		auths, err := defaultAuths(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not gather authentication details")
		}

		authConfig := &authn.AuthConfig{}
		// var transport *http.Transport
		transport := http.DefaultTransport.(*http.Transport).Clone()

		// Annoyingly convert between regtypes and authn.
		if auth, ok := auths[ocipack.ref.Context().RegistryStr()]; ok {
			authConfig.Username = auth.User
			authConfig.Password = auth.Token

			if !auth.VerifySSL {
				// transport = http.DefaultTransport.(*http.Transport).Clone()
				transport.TLSClientConfig = &tls.Config{
					InsecureSkipVerify: true,
				}
			}
		}

		v1ImageIndex, err := remote.Index(ocipack.ref,
			remote.WithAuth(&simpleauth.SimpleAuthenticator{
				Auth: authConfig,
			}),
			remote.WithTransport(transport),
		)
		if err != nil {
			return nil, fmt.Errorf("could not get index from registry: %v", err)
		}

		ocipack.index, err = NewIndex(ctx, handle)
		if err != nil {
			return nil, err
		}

		v1ImageIndexManifest, err := v1ImageIndex.IndexManifest()
		if err != nil {
			return nil, fmt.Errorf("could not access index manifest: %w", err)
		}

		index, err := FromGoogleV1IndexManifestToOCISpec(*v1ImageIndexManifest)
		if err != nil {
			return nil, fmt.Errorf("could not convert index manifest: %w", err)
		}

		for _, descriptor := range index.Manifests {
			descriptor := descriptor

			manifest, err := NewManifest(ctx, handle)
			if err != nil {
				return nil, fmt.Errorf("could not instantiate new manifest: %w", err)
			}

			manifest.desc = &descriptor
			manifest.config.Architecture = descriptor.Platform.Architecture
			manifest.config.Platform = *descriptor.Platform
			manifest.manifest.Config.Platform = descriptor.Platform
			ocipack.index.manifests = append(ocipack.index.manifests, manifest)

			if manifest.desc.Digest.String() == dgst.String() {
				ocipack.manifest = manifest
			}
		}

		if ocipack.manifest == nil {
			return nil, fmt.Errorf("remote index does not contain digest '%s'", dgst.String())
		}
	}

	architecture, err := arch.TransformFromSchema(ctx,
		ocipack.manifest.manifest.Config.Platform.Architecture,
	)
	if err != nil {
		return nil, err
	}

	ocipack.arch = architecture.(arch.Architecture)

	platform, err := plat.TransformFromSchema(ctx,
		ocipack.manifest.manifest.Config.Platform.OS,
	)
	if err != nil {
		return nil, err
	}

	ocipack.plat = platform.(plat.Platform)

	ocipack.kconfig = kconfig.KeyValueMap{}
	for _, feature := range ocipack.manifest.config.OSFeatures {
		_, kval := kconfig.NewKeyValue(feature)
		ocipack.kconfig.Override(kval)
	}

	return &ocipack, nil
}

// Type implements unikraft.Nameable
func (ocipack *ociPackage) Type() unikraft.ComponentType {
	return unikraft.ComponentTypeApp
}

// Name implements unikraft.Nameable
func (ocipack *ociPackage) Name() string {
	return ocipack.ref.Context().Name()
}

// Name implements fmt.Stringer
func (ocipack *ociPackage) String() string {
	return ocipack.imageRef()
}

// Version implements unikraft.Nameable
func (ocipack *ociPackage) Version() string {
	return ocipack.ref.Identifier()
}

// imageRef returns the OCI-standard image name in the format `name:tag`
func (ocipack *ociPackage) imageRef() string {
	if strings.HasPrefix(ocipack.Version(), "sha256:") {
		return fmt.Sprintf("%s@%s", ocipack.Name(), ocipack.Version())
	}
	return fmt.Sprintf("%s:%s", ocipack.Name(), ocipack.Version())
}

// Metadata implements pack.Package
func (ocipack *ociPackage) Metadata() interface{} {
	return ocipack.manifest.manifest
}

// Columns implements pack.Package
func (ocipack *ociPackage) Columns() []tableprinter.Column {
	return []tableprinter.Column{
		{Name: "digest", Value: ocipack.manifest.desc.Digest.String()[7:14]},
		{Name: "plat", Value: fmt.Sprintf("%s/%s", ocipack.Platform().Name(), ocipack.Architecture().Name())},
	}
}

// Push implements pack.Package
func (ocipack *ociPackage) Push(ctx context.Context, opts ...pack.PushOption) error {
	desc, err := ocipack.index.Descriptor()
	if err != nil {
		return err
	}

	if err := ocipack.handle.PushDescriptor(ctx, ocipack.imageRef(), desc); err != nil {
		return err
	}

	return nil
}

// Pull implements pack.Package
func (ocipack *ociPackage) Pull(ctx context.Context, opts ...pack.PullOption) error {
	popts, err := pack.NewPullOptions(opts...)
	if err != nil {
		return err
	}

	// Check that the manifest has been fully resolved or pull the descriptor
	// which will fetch all associated descriptors.
	if _, err := ocipack.handle.ResolveManifest(ctx,
		ocipack.imageRef(),
		ocipack.manifest.desc.Digest,
	); err != nil {
		// Pull the index but set the platform such that the relevant manifests can
		// be retrieved as well.
		if err := ocipack.handle.PullDigest(
			ctx,
			ocispec.MediaTypeImageIndex,
			ocipack.imageRef(),
			ocipack.manifest.desc.Digest,
			ocipack.manifest.desc.Platform,
			popts.OnProgress,
		); err != nil {
			return err
		}
	}

	// Unpack the image if a working directory has been provided
	if len(popts.Workdir()) > 0 {
		image, err := ocipack.handle.UnpackImage(ctx,
			ocipack.imageRef(),
			ocipack.manifest.desc.Digest,
			popts.Workdir(),
		)
		if err != nil {
			return err
		}

		// Set the kernel, since it is a well-known within the destination path
		ocipack.kernel = filepath.Join(popts.Workdir(), WellKnownKernelPath)

		// Set the command
		ocipack.command = image.Config.Cmd

		// Set the initrd if available
		initrdPath := filepath.Join(popts.Workdir(), WellKnownInitrdPath)
		if f, err := os.Stat(initrdPath); err == nil && f.Size() > 0 {
			ocipack.initrd, err = initrd.New(ctx, initrdPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Delete implements pack.Package.
func (ocipack *ociPackage) Delete(ctx context.Context) error {
	if err := ocipack.handle.DeleteManifest(ctx, ocipack.imageRef(), ocipack.manifest.desc.Digest); err != nil {
		return fmt.Errorf("could not delete package manifest: %w", err)
	}

	indexDesc, err := ocipack.handle.ResolveIndex(ctx, ocipack.imageRef())
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("could not resolve index: %w", err)
	} else if indexDesc == nil {
		return nil
	}

	var manifests []ocispec.Descriptor

	for _, manifest := range indexDesc.Manifests {
		if manifest.Digest.String() == ocipack.manifest.desc.Digest.String() {
			continue
		}

		manifests = append(manifests, manifest)
	}

	if len(manifests) == 0 {
		return ocipack.handle.DeleteIndex(ctx, ocipack.imageRef())
	}

	indexDesc.Manifests = manifests

	newIndex, err := NewIndexFromSpec(ctx, ocipack.handle, indexDesc)
	if err != nil {
		return fmt.Errorf("could not prepare new index: %w", err)
	}

	_, err = newIndex.Save(ctx, ocipack.imageRef(), nil)
	return err
}

// Pull implements pack.Package
func (ocipack *ociPackage) Format() pack.PackageFormat {
	return OCIFormat
}

// Source implements unikraft.target.Target
func (ocipack *ociPackage) Source() string {
	return ""
}

// Path implements unikraft.target.Target
func (ocipack *ociPackage) Path() string {
	return ""
}

// KConfigTree implements unikraft.target.Target
func (ocipack *ociPackage) KConfigTree(context.Context, ...*kconfig.KeyValue) (*kconfig.KConfigFile, error) {
	return nil, fmt.Errorf("not implemented: oci.ociPackage.KConfigTree")
}

// KConfig implements unikraft.target.Target
func (ocipack *ociPackage) KConfig() kconfig.KeyValueMap {
	return ocipack.kconfig
}

// PrintInfo implements unikraft.target.Target
func (ocipack *ociPackage) PrintInfo(context.Context) string {
	return "not implemented: oci.ociPackage.PrintInfo"
}

// Architecture implements unikraft.target.Target
func (ocipack *ociPackage) Architecture() arch.Architecture {
	return ocipack.arch
}

// Platform implements unikraft.target.Target
func (ocipack *ociPackage) Platform() plat.Platform {
	return ocipack.plat
}

// Kernel implements unikraft.target.Target
func (ocipack *ociPackage) Kernel() string {
	return ocipack.kernel
}

// KernelDbg implements unikraft.target.Target
func (ocipack *ociPackage) KernelDbg() string {
	return ocipack.kernel
}

// Initrd implements unikraft.target.Target
func (ocipack *ociPackage) Initrd() initrd.Initrd {
	return ocipack.initrd
}

// Command implements unikraft.target.Target
func (ocipack *ociPackage) Command() []string {
	return ocipack.command
}

// ConfigFilename implements unikraft.target.Target
func (ocipack *ociPackage) ConfigFilename() string {
	return ""
}

// MarshalYAML implements unikraft.target.Target (yaml.Marshaler)
func (ocipack *ociPackage) MarshalYAML() (interface{}, error) {
	if ocipack == nil {
		return nil, nil
	}

	return map[string]interface{}{
		"architecture": ocipack.arch.Name(),
		"platform":     ocipack.plat.Name(),
	}, nil
}
